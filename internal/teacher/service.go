package teacher

import (
	"context"

	"github.com/yohannes/chines-backend/internal/auth"
	"github.com/yohannes/chines-backend/internal/class"
	"github.com/yohannes/chines-backend/internal/platform/apperr"
)

type Service struct {
	repo    *Repository
	classes *class.Repository
}

func NewService(repo *Repository, classes *class.Repository) *Service {
	return &Service{repo: repo, classes: classes}
}

func (s *Service) Overview(ctx context.Context, id auth.Identity) (*Overview, error) {
	return s.repo.Overview(ctx, id.UserID, id.Role)
}

func (s *Service) ClassProgress(ctx context.Context, id auth.Identity, classID string) (*ClassProgress, error) {
	if err := s.mustManage(ctx, id, classID); err != nil {
		return nil, err
	}
	c, err := s.classes.Get(ctx, classID)
	if err != nil {
		return nil, err
	}
	students, err := s.repo.ClassProgress(ctx, classID)
	if err != nil {
		return nil, err
	}
	return &ClassProgress{ClassID: c.ID, ClassName: c.Name, Students: students}, nil
}

func (s *Service) StudentDetail(ctx context.Context, id auth.Identity, classID, studentID string) (*StudentDetail, error) {
	if err := s.mustManage(ctx, id, classID); err != nil {
		return nil, err
	}
	return s.repo.StudentDetail(ctx, classID, studentID)
}

func (s *Service) mustManage(ctx context.Context, id auth.Identity, classID string) error {
	ok, err := s.classes.Access(ctx, classID, id.UserID, id.Role, true)
	if err != nil {
		return err
	}
	if !ok {
		return apperr.ErrNotFound
	}
	return nil
}
