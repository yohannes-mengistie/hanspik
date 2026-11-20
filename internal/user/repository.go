package user

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

const selectUser = `SELECT id::text, name, email, password_hash, role, disabled_at, created_at FROM users`

func (r *Repository) scanOne(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.DisabledAt, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *Repository) Create(ctx context.Context, name, email, hash string, role Role) (*User, error) {
	const q = `
		INSERT INTO users (name, email, password_hash, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id::text, created_at`

	u := &User{
		Name:         name,
		Email:        email,
		PasswordHash: hash,
		Role:         role,
	}
	err := r.db.QueryRow(ctx, q, name, email, hash, string(role)).Scan(&u.ID, &u.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrEmailTaken
		}
		return nil, err
	}
	return u, nil
}

func (r *Repository) GetByEmail(ctx context.Context, email string) (*User, error) {
	return r.scanOne(r.db.QueryRow(ctx, selectUser+" WHERE email = $1", email))
}

func (r *Repository) GetByID(ctx context.Context, id string) (*User, error) {
	return r.scanOne(r.db.QueryRow(ctx, selectUser+" WHERE id = $1", id))
}

// List returns one page of users plus the total matching the same filter.
func (r *Repository) List(ctx context.Context, role, q string, page, pageSize int) ([]User, int, error) {
	const where = `
		WHERE ($1 = '' OR role = $1)
		  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR email ILIKE '%' || $2 || '%')`

	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM users`+where, role, q).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	rows, err := r.db.Query(ctx,
		selectUser+where+` ORDER BY created_at DESC LIMIT $3 OFFSET $4`,
		role, q, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	users := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.DisabledAt, &u.CreatedAt); err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	return users, total, rows.Err()
}

// SetPassword replaces a user's password hash (used by admin bootstrap and
// password resets). Callers must hash before passing the value in.
func (r *Repository) SetPassword(ctx context.Context, id, hash string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1`, id, hash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) SetRole(ctx context.Context, id string, role Role) error {
	tag, err := r.db.Exec(ctx, `UPDATE users SET role = $2, updated_at = now() WHERE id = $1`, id, string(role))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetDisabled toggles users.disabled_at. Passing true sets it to now,
// passing false clears it.
func (r *Repository) SetDisabled(ctx context.Context, id string, disabled bool) error {
	var tag pgconn.CommandTag
	var err error
	if disabled {
		tag, err = r.db.Exec(ctx,
			`UPDATE users SET disabled_at = now(), updated_at = now() WHERE id = $1`, id)
	} else {
		tag, err = r.db.Exec(ctx,
			`UPDATE users SET disabled_at = NULL, updated_at = now() WHERE id = $1`, id)
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) CountAdmins(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&n)
	return n, err
}
