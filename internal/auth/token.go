package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/yohannes/chines-backend/internal/user"
)

var ErrInvalidToken = errors.New("invalid or expired token")

type Claims struct {
	Role user.Role `json:"role"`
	jwt.RegisteredClaims
}

type TokenManager struct {
	secret    []byte
	accessTTL time.Duration
}

func NewTokenManager(secret string, accessTTL time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), accessTTL: accessTTL}
}

func (m *TokenManager) NewAccessToken(u *user.User) (string, error) {
	now := time.Now()
	claims := Claims{
		Role: u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

func (m *TokenManager) Parse(raw string) (*Claims, error) {
	claims := &Claims{}
	tok, err := jwt.ParseWithClaims(raw, claims,
		func(t *jwt.Token) (any, error) { return m.secret, nil },
		jwt.WithValidMethods([]string{"HS256"}), // blocks the "alg: none" attack
	)
	if err != nil || !tok.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
