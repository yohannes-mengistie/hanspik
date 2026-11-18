package practice

import (
	"context"
	"time"

	"github.com/yohannes/chines-backend/internal/auth"
	"github.com/yohannes/chines-backend/internal/class"
	"github.com/yohannes/chines-backend/internal/lesson"
	"github.com/yohannes/chines-backend/internal/platform/apperr"
)

const dateLayout = "2006-01-02"

var validTypes = map[string]bool{"flashcard": true, "choice": true, "typing": true}

type Service struct {
	repo    *Repository
	lessons *lesson.Repository
	classes *class.Repository
}

func NewService(repo *Repository, lessons *lesson.Repository, classes *class.Repository) *Service {
	return &Service{repo: repo, lessons: lessons, classes: classes}
}

func parseDate(s string) (time.Time, error) {
	d, err := time.Parse(dateLayout, s)
	if err != nil {
		return time.Time{}, apperr.Validation("date must look like 2026-11-17")
	}
	return d, nil
}

func (s *Service) Due(ctx context.Context, id auth.Identity, date string, limit int) ([]Item, error) {
	today, err := parseDate(date)
	if err != nil {
		return nil, err
	}
	return s.repo.Due(ctx, id.UserID, today, limit)
}

func (s *Service) Summary(ctx context.Context, id auth.Identity, date string) (*Summary, error) {
	today, err := parseDate(date)
	if err != nil {
		return nil, err
	}
	return s.repo.Summary(ctx, id.UserID, today)
}

func (s *Service) Answer(ctx context.Context, id auth.Identity, wordID, typ string, quality int) (*Progress, error) {
	if !validTypes[typ] {
		return nil, apperr.Validation("type must be flashcard, choice, or typing")
	}
	if quality < 0 || quality > 5 {
		return nil, apperr.Validation("quality must be between 0 and 5")
	}

	// The student may only answer words from a class they belong to.
	classID, err := s.lessons.ClassIDOfWord(ctx, wordID)
	if err != nil {
		return nil, err // ErrNotFound if the word doesn't exist
	}
	ok, err := s.classes.Access(ctx, classID, id.UserID, id.Role, false)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, apperr.ErrNotFound
	}

	return s.repo.Answer(ctx, id.UserID, wordID, typ, quality)
}