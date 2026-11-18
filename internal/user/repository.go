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

const selectUser = `SELECT id::text, name, email, password_hash, role, created_at FROM users`

func (r *Repository) scanOne(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *Repository) Create(ctx context.Context, name , email , hash string, role Role) (*User,error) {
		const q = `
		INSERT INTO users (name, email, password_hash, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id::text, created_at`

		u := &User{
			Name: name,
			Email: email,
			PasswordHash: hash,
			Role: role,
		}
		err := r.db.QueryRow(ctx, q,name , email,hash, string(role)).Scan(&u.ID, &u.CreatedAt)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err,&pgErr) && pgErr.Code == "23505" {
				return nil, ErrEmailTaken
			}
			return nil, err
		}
		return u , nil
} 
func (r *Repository) GetByEmail(ctx context.Context, email string) (*User,error) {
		return r.scanOne(r.db.QueryRow(ctx, selectUser + " WHERE email = $1", email))
}

func (r *Repository) GetByID(ctx context.Context, id string) (*User,error) {
		return r.scanOne(r.db.QueryRow(ctx, selectUser + " WHERE id = $1", id))
}

