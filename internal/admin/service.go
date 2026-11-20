package admin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/mail"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/yohannes/chines-backend/internal/auth"
	"github.com/yohannes/chines-backend/internal/platform/apperr"
	"github.com/yohannes/chines-backend/internal/user"
)

type Service struct {
	users  *user.Repository
	tokens *auth.Repository
	repo   *Repository
}

func NewService(users *user.Repository, tokens *auth.Repository, repo *Repository) *Service {
	return &Service{users: users, tokens: tokens, repo: repo}
}

func (s *Service) Overview(ctx context.Context) (*Overview, error) {
	return s.repo.Overview(ctx)
}

func (s *Service) ListUsers(ctx context.Context, role, q string, page, pageSize int) (*UserPage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		return nil, apperr.Validation("page_size must be between 1 and 100")
	}
	if role != "" && !user.Role(role).Valid() {
		return nil, apperr.Validation("role must be student, teacher, or admin")
	}
	users, total, err := s.users.List(ctx, role, strings.TrimSpace(q), page, pageSize)
	if err != nil {
		return nil, err
	}
	return &UserPage{Users: users, Page: page, PageSize: pageSize, Total: total}, nil
}

type CreateTeacherInput struct {
	Name, Email, Password string
}

func (s *Service) CreateTeacher(ctx context.Context, in CreateTeacherInput) (*CreateUserResult, error) {
	name := strings.TrimSpace(in.Name)
	email := strings.ToLower(strings.TrimSpace(in.Email))

	if name == "" {
		return nil, apperr.Validation("name is required")
	}
	if addr, err := mail.ParseAddress(email); err != nil || addr.Address != email {
		return nil, apperr.Validation("email is not valid")
	}

	password := in.Password
	if password == "" {
		generated, err := tempPassword()
		if err != nil {
			return nil, err
		}
		password = generated
	}
	if len(password) < 8 || len(password) > 72 {
		return nil, apperr.Validation("password must be 8 to 72 characters")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	u, err := s.users.Create(ctx, name, email, string(hash), user.RoleTeacher)
	if err != nil {
		return nil, err
	}
	return &CreateUserResult{User: u, TemporaryPwd: password}, nil
}

func (s *Service) ChangeRole(ctx context.Context, actor auth.Identity, targetID, role string) error {
	if !user.Role(role).Valid() {
		return apperr.Validation("role must be student, teacher, or admin")
	}
	target, err := s.users.GetByID(ctx, targetID)
	if err != nil {
		return err
	}
	if target.ID == actor.UserID && user.Role(role) != user.RoleAdmin {
		return apperr.Validation("you cannot demote your own admin account")
	}
	if target.Role == user.RoleAdmin && user.Role(role) != user.RoleAdmin {
		if err := s.ensureAnotherAdmin(ctx); err != nil {
			return err
		}
	}
	if err := s.users.SetRole(ctx, targetID, user.Role(role)); err != nil {
		return err
	}
	// Force the next refresh to pick up the new role immediately.
	return s.tokens.RevokeAllForUser(ctx, targetID)
}

func (s *Service) SetDisabled(ctx context.Context, actor auth.Identity, targetID string, disabled bool) error {
	target, err := s.users.GetByID(ctx, targetID)
	if err != nil {
		return err
	}
	if target.ID == actor.UserID {
		return apperr.Validation("you cannot disable your own account")
	}
	if disabled && target.Role == user.RoleAdmin {
		if err := s.ensureAnotherAdmin(ctx); err != nil {
			return err
		}
	}
	if err := s.users.SetDisabled(ctx, targetID, disabled); err != nil {
		return err
	}
	if disabled {
		// A disabled user must not be able to mint a fresh access token.
		return s.tokens.RevokeAllForUser(ctx, targetID)
	}
	return nil
}

func (s *Service) ensureAnotherAdmin(ctx context.Context) error {
	n, err := s.users.CountAdmins(ctx)
	if err != nil {
		return err
	}
	if n <= 1 {
		return apperr.Validation("the last admin cannot be removed")
	}
	return nil
}

func tempPassword() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Sentinel kept here so handler can map errors if needed.
var ErrNoChange = errors.New("no change")
