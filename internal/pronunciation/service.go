package pronunciation

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/yohannes/chines-backend/internal/auth"
	"github.com/yohannes/chines-backend/internal/class"
	"github.com/yohannes/chines-backend/internal/lesson"
	"github.com/yohannes/chines-backend/internal/platform/apperr"
)

// Audio is validated by magic bytes, never by filename or Content-Type.
var audioExt = map[string]string{
	"audio/wave":      ".wav",
	"audio/wav":       ".wav",
	"audio/x-wav":     ".wav",
	"audio/mpeg":      ".mp3",
	"application/ogg": ".ogg",
	"audio/ogg":       ".ogg",
}

// FileStore is private storage: no public URL is ever derived from it.
type FileStore interface {
	Save(ctx context.Context, name string, r io.Reader) error
	Open(ctx context.Context, name string) (io.ReadCloser, error)
	Delete(ctx context.Context, name string) error
}

type Service struct {
	repo    *Repository
	lessons *lesson.Repository
	classes *class.Repository
	store   FileStore
	scorer  Scorer
	limiter *Limiter
}

func NewService(repo *Repository, lessons *lesson.Repository, classes *class.Repository,
	store FileStore, scorer Scorer, limiter *Limiter) *Service {
	if limiter == nil {
		limiter = NewLimiter(12, time.Minute) // 12 attempts per minute
	}
	return &Service{
		repo:    repo,
		lessons: lessons,
		classes: classes,
		store:   store,
		scorer:  scorer,
		limiter: limiter,
	}
}

// Score validates access, stores the recording privately, calls the provider
// and records the attempt. A failed provider call leaves no file behind.
func (s *Service) Score(ctx context.Context, id auth.Identity, wordID string, file io.Reader) (*Attempt, error) {
	word, err := s.lessons.GetWord(ctx, wordID)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeStudent(ctx, id, wordID); err != nil {
		return nil, err
	}
	if !s.limiter.Allow(id.UserID) {
		return nil, ErrRateLimited
	}

	br := bufio.NewReader(io.LimitReader(file, MaxAudioBytes+1))
	head, err := br.Peek(512)
	if err != nil && err != io.EOF {
		return nil, err
	}
	ext, ok := audioExt[http.DetectContentType(head)]
	if !ok {
		return nil, ErrUnsupportedAudio
	}

	audio, err := io.ReadAll(io.LimitReader(br, MaxAudioBytes+1))
	if err != nil {
		return nil, err
	}
	if len(audio) == 0 {
		return nil, apperr.Validation("audio file is empty")
	}
	if len(audio) > MaxAudioBytes {
		return nil, apperr.Validation("file too large (max 2 MB)")
	}

	name := uuid.NewString() + ext
	if err := s.store.Save(ctx, name, bytes.NewReader(audio)); err != nil {
		return nil, err
	}
	removeFile := func() {
		if err := s.store.Delete(ctx, name); err != nil {
			log.Printf("pronunciation: could not delete %s: %v", name, err)
		}
	}

	result, err := s.scorer.Score(ctx, audio, word.Hanzi, "zh-CN")
	if err != nil {
		removeFile()
		return nil, err
	}

	attempt, err := s.repo.Create(ctx, &Attempt{
		StudentID:    id.UserID,
		WordID:       wordID,
		AudioPath:    name,
		Accuracy:     result.Accuracy,
		Fluency:      result.Fluency,
		Completeness: result.Completeness,
		Tone:         result.Tone,
		Overall:      result.Overall,
		Details:      result.Details,
		Provider:     result.Provider,
	})
	if err != nil {
		removeFile()
		return nil, err
	}
	return attempt, nil
}

// History returns this student's attempts for one word, newest first.
func (s *Service) History(ctx context.Context, id auth.Identity, wordID string, limit int) ([]Attempt, error) {
	if _, err := s.lessons.GetWord(ctx, wordID); err != nil {
		return nil, err
	}
	if err := s.authorizeStudent(ctx, id, wordID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	return s.repo.ListByWord(ctx, id.UserID, wordID, limit)
}

// OpenAudio lets the owner, their teacher, and an admin stream an attempt.
// Everyone else gets 404 so the existence of the recording is not revealed.
func (s *Service) OpenAudio(ctx context.Context, id auth.Identity, attemptID string) (io.ReadCloser, string, error) {
	attempt, classID, err := s.repo.Get(ctx, attemptID)
	if err != nil {
		return nil, "", err
	}
	if attempt.StudentID != id.UserID {
		ok, err := s.classes.Access(ctx, classID, id.UserID, id.Role, true)
		if err != nil {
			return nil, "", err
		}
		if !ok {
			return nil, "", apperr.ErrNotFound
		}
	}
	file, err := s.store.Open(ctx, attempt.AudioPath)
	if err != nil {
		return nil, "", apperr.ErrNotFound
	}
	return file, filepath.Ext(attempt.AudioPath), nil
}

func (s *Service) authorizeStudent(ctx context.Context, id auth.Identity, wordID string) error {
	classID, err := s.lessons.ClassIDOfWord(ctx, wordID)
	if err != nil {
		return err
	}
	ok, err := s.classes.Access(ctx, classID, id.UserID, id.Role, false)
	if err != nil {
		return err
	}
	if !ok {
		return apperr.ErrNotFound
	}
	return nil
}
