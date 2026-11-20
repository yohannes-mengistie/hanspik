package auth

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/yohannes/chines-backend/internal/user"
)

var ErrInvalidCredentials = errors.New("invalid email or password")

type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

// Used so a wrong email takes as long as a wrong password (prevents user enumeration by timing).
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password"), bcrypt.DefaultCost)

type Service struct {
	users      *user.Repository
	repo       *Repository
	tokens     *TokenManager
	refreshTTL time.Duration
}

func NewService(users *user.Repository, repo *Repository, tokens *TokenManager, refreshTTL time.Duration) *Service {
	return &Service{users: users, repo: repo, tokens: tokens, refreshTTL: refreshTTL}
}

type RegisterInput struct{ Name, Email, Password string }

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"` // access token lifetime in seconds
}

type AuthResult struct {
	User   *user.User `json:"user"`
	Tokens *TokenPair `json:"tokens"`
}

func (s *Service) Register(ctx context.Context, in RegisterInput) (*AuthResult, error) {
	name := strings.TrimSpace(in.Name)
	email := strings.ToLower(strings.TrimSpace(in.Email))

	if name == "" {
		return nil, &ValidationError{"name is required"}
	}
	if addr, err := mail.ParseAddress(email); err != nil || addr.Address != email {
		return nil, &ValidationError{"email is not valid"}
	}
	// bcrypt only uses the first 72 bytes, so longer passwords are rejected.
	if len(in.Password) < 8 || len(in.Password) > 72 {
		return nil, &ValidationError{"password must be 8 to 72 characters"}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// Self-registration is always a student.
	u, err := s.users.Create(ctx, name, email, string(hash), user.RoleStudent)
	if err != nil {
		return nil, err
	}

	pair, err := s.issue(ctx, u)
	if err != nil {
		return nil, err
	}
	return &AuthResult{User: u, Tokens: pair}, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (*AuthResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	u, err := s.users.GetByEmail(ctx, email)
	if errors.Is(err, user.ErrNotFound) {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if u.DisabledAt != nil {
		return nil, ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return nil, ErrInvalidCredentials
	}

	pair, err := s.issue(ctx, u)
	if err != nil {
		return nil, err
	}
	return &AuthResult{User: u, Tokens: pair}, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	userID, err := s.repo.Consume(ctx, hashToken(refreshToken))
	if err != nil {
		return nil, err
	}
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u.DisabledAt != nil {
		return nil, ErrInvalidToken
	}
	return s.issue(ctx, u)
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	return s.repo.Revoke(ctx, hashToken(refreshToken))
}

func (s *Service) Me(ctx context.Context, userID string) (*user.User, error) {
	return s.users.GetByID(ctx, userID)
}

func (s *Service) issue(ctx context.Context, u *user.User) (*TokenPair, error) {
	access, err := s.tokens.NewAccessToken(u)
	if err != nil {
		return nil, err
	}
	raw, hash, err := newRefreshToken()
	if err != nil {
		return nil, err
	}
	if err := s.repo.Save(ctx, u.ID, hash, time.Now().Add(s.refreshTTL)); err != nil {
		return nil, err
	}
	return &TokenPair{
		AccessToken:  access,
		RefreshToken: raw,
		ExpiresIn:    int(s.tokens.accessTTL.Seconds()),
	}, nil
}
