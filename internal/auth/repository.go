package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// newRefreshToken returns the raw token (sent to the client) and its hash (stored).
func newRefreshToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, hashToken(raw), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (r *Repository) Save(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		userID, tokenHash, expiresAt)
	return err
}

// Consume revokes the token and returns its owner, in ONE atomic statement.
// If two requests use the same token at once, only one succeeds.
func (r *Repository) Consume(ctx context.Context, tokenHash string) (string, error) {
	const q = `
		UPDATE refresh_tokens
		SET revoked_at = now()
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > now()
		RETURNING user_id::text`

	var userID string
	err := r.db.QueryRow(ctx, q, tokenHash).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrInvalidToken
	}
	return userID, err
}

func (r *Repository) Revoke(ctx context.Context, tokenHash string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`,
		tokenHash)
	return err
}

// RevokeAllForUser signs a user out of every device. Used when a user is
// disabled or has their role changed, so the next refresh cannot restore an
// old privilege.
func (r *Repository) RevokeAllForUser(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = now()
		 WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}
