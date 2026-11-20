package pronunciation

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yohannes/chines-backend/internal/platform/apperr"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

const attemptCols = `id::text, student_id::text, word_id::text, audio_path,
	accuracy_score, fluency_score, completeness_score, tone_score,
	overall_score, details, provider, created_at`

func scanAttempt(s interface{ Scan(...any) error }) (*Attempt, error) {
	var a Attempt
	var details []byte
	err := s.Scan(&a.ID, &a.StudentID, &a.WordID, &a.AudioPath,
		&a.Accuracy, &a.Fluency, &a.Completeness, &a.Tone,
		&a.Overall, &details, &a.Provider, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	a.Details = details
	return &a, nil
}

func (r *Repository) Create(ctx context.Context, a *Attempt) (*Attempt, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO pronunciation_attempts
			(student_id, word_id, audio_path, accuracy_score, fluency_score,
			 completeness_score, tone_score, overall_score, details, provider)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10)
		RETURNING `+attemptCols,
		a.StudentID, a.WordID, a.AudioPath, a.Accuracy, a.Fluency,
		a.Completeness, a.Tone, a.Overall, string(a.Details), a.Provider)
	return scanAttempt(row)
}

func (r *Repository) ListByWord(ctx context.Context, studentID, wordID string, limit int) ([]Attempt, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+attemptCols+`
		FROM pronunciation_attempts
		WHERE student_id = $1 AND word_id = $2
		ORDER BY created_at DESC
		LIMIT $3`, studentID, wordID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Attempt{}
	for rows.Next() {
		a, err := scanAttempt(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

// Get returns the attempt and the class it belongs to, for permission checks.
func (r *Repository) Get(ctx context.Context, id string) (*Attempt, string, error) {
	row := r.db.QueryRow(ctx, `
		SELECT `+attemptCols+`, l.class_id::text
		FROM pronunciation_attempts pa
		JOIN words w ON w.id = pa.word_id
		JOIN lessons l ON l.id = w.lesson_id
		WHERE pa.id = $1`, id)

	var a Attempt
	var details []byte
	var classID string
	err := row.Scan(&a.ID, &a.StudentID, &a.WordID, &a.AudioPath,
		&a.Accuracy, &a.Fluency, &a.Completeness, &a.Tone,
		&a.Overall, &details, &a.Provider, &a.CreatedAt, &classID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", apperr.ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}
	a.Details = details
	return &a, classID, nil
}
