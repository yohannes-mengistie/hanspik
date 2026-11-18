package class

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/yohannes/chines-backend/internal/auth"
	"github.com/yohannes/chines-backend/internal/platform/apperr"
	"github.com/yohannes/chines-backend/internal/user"
)

type Service struct {
	repo  *Repository
	users *user.Repository
}

func NewService(repo *Repository, users *user.Repository) *Service {
	return &Service{repo: repo, users: users}
}

func (s *Service) Create(ctx context.Context, id auth.Identity, name string) (*Class, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 100 {
		return nil, apperr.Validation("name is required (max 100 characters)")
	}
	return s.repo.Create(ctx, name, id.UserID)
}

func (s *Service) List(ctx context.Context, id auth.Identity) ([]Class, error) {
	return s.repo.ListFor(ctx, id.UserID, id.Role)
}

func (s *Service) Members(ctx context.Context, id auth.Identity, classID string) ([]Member, error) {
	if err := s.mustManage(ctx, id, classID); err != nil {
		return nil, err
	}
	return s.repo.ListMembers(ctx, classID)
}

func (s *Service) AddStudent(ctx context.Context, id auth.Identity, classID, email string) (*Member, error) {
	if err := s.mustManage(ctx, id, classID); err != nil {
		return nil, err
	}

	email = strings.ToLower(strings.TrimSpace(email))
	u, err := s.users.GetByEmail(ctx, email)
	if errors.Is(err, user.ErrNotFound) {
		return nil, apperr.Validation("no registered user with this email")
	}
	if err != nil {
		return nil, err
	}
	if u.Role != user.RoleStudent {
		return nil, apperr.Validation("this user is not a student")
	}

	if err := s.repo.AddMember(ctx, classID, u.ID); err != nil {
		return nil, err
	}
	return &Member{ID: u.ID, Name: u.Name, Email: u.Email}, nil
}

func (s *Service) mustManage(ctx context.Context, id auth.Identity, classID string) error {
	ok, err := s.repo.Access(ctx, classID, id.UserID, id.Role, true)
	if err != nil {
		return err
	}
	if !ok {
		return apperr.ErrNotFound
	}
	return nil
}