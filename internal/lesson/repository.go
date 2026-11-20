package lesson

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yohannes/chines-backend/internal/platform/apperr"
	"github.com/yohannes/chines-backend/internal/user"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// scanner is satisfied by both pgx.Row and pgx.Rows.
type scanner interface{ Scan(dest ...any) error }

const lessonCols = `l.id::text, l.class_id::text, l.title, to_char(l.lesson_date, 'YYYY-MM-DD'), l.created_at`
const wordCols = `id::text, lesson_id::text, hanzi, pinyin, meaning, example_sentence, audio_url, position`

func scanLesson(s scanner) (*Lesson, error) {
	var l Lesson
	if err := s.Scan(&l.ID, &l.ClassID, &l.Title, &l.LessonDate, &l.CreatedAt); err != nil {
		return nil, err
	}
	return &l, nil
}

func scanWord(s scanner) (*Word, error) {
	var w Word
	err := s.Scan(&w.ID, &w.LessonID, &w.Hanzi, &w.Pinyin, &w.Meaning, &w.ExampleSentence, &w.AudioURL, &w.Position)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *Repository) CreateLesson(ctx context.Context, classID, title string, date time.Time) (*Lesson, error) {
	row := r.db.QueryRow(ctx, `
		WITH l AS (
			INSERT INTO lessons (class_id, title, lesson_date) VALUES ($1, $2, $3) RETURNING *
		)
		SELECT `+lessonCols+` FROM l`, classID, title, date)
	return scanLesson(row)
}

func (r *Repository) GetLesson(ctx context.Context, id string) (*Lesson, error) {
	l, err := scanLesson(r.db.QueryRow(ctx, `SELECT `+lessonCols+` FROM lessons l WHERE l.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.ErrNotFound
	}
	return l, err
}

func (r *Repository) ListByClass(ctx context.Context, classID string) ([]Lesson, error) {
	return r.queryLessons(ctx,
		`SELECT `+lessonCols+` FROM lessons l WHERE l.class_id = $1
		 ORDER BY l.lesson_date DESC, l.created_at DESC`, classID)
}

// ListByDate returns the lessons of one day, across all classes the user can see.
func (r *Repository) ListByDate(ctx context.Context, userID string, role user.Role, date time.Time) ([]Lesson, error) {
	const base = `SELECT ` + lessonCols + ` FROM lessons l `
	const tail = ` ORDER BY l.created_at`

	switch role {
	case user.RoleAdmin:
		return r.queryLessons(ctx, base+`WHERE l.lesson_date = $1`+tail, date)
	case user.RoleTeacher:
		return r.queryLessons(ctx, base+
			`JOIN classes c ON c.id = l.class_id AND c.teacher_id = $1
			 WHERE l.lesson_date = $2`+tail, userID, date)
	default:
		return r.queryLessons(ctx, base+
			`JOIN class_members m ON m.class_id = l.class_id AND m.student_id = $1
			 WHERE l.lesson_date = $2`+tail, userID, date)
	}
}

func (r *Repository) queryLessons(ctx context.Context, q string, args ...any) ([]Lesson, error) {
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	lessons := []Lesson{}
	for rows.Next() {
		l, err := scanLesson(rows)
		if err != nil {
			return nil, err
		}
		lessons = append(lessons, *l) // *l copies the value out of the pointer
	}
	return lessons, rows.Err()
}

func (r *Repository) ListWords(ctx context.Context, lessonID string) ([]Word, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+wordCols+` FROM words WHERE lesson_id = $1 ORDER BY position`, lessonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	words := []Word{}
	for rows.Next() {
		w, err := scanWord(rows)
		if err != nil {
			return nil, err
		}
		words = append(words, *w)
	}
	return words, rows.Err()
}

func (r *Repository) AddWord(ctx context.Context, lessonID string, w Word) (*Word, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO words (lesson_id, hanzi, pinyin, meaning, example_sentence, position)
		VALUES ($1, $2, $3, $4, $5,
			(SELECT COALESCE(MAX(position), 0) + 1 FROM words WHERE lesson_id = $1))
		RETURNING `+wordCols,
		lessonID, w.Hanzi, w.Pinyin, w.Meaning, w.ExampleSentence)
	return scanWord(row)
}

// ClassIDOfWord finds which class a word belongs to (for permission checks).
func (r *Repository) ClassIDOfWord(ctx context.Context, wordID string) (string, error) {
	var classID string
	err := r.db.QueryRow(ctx, `
		SELECT l.class_id::text FROM words w JOIN lessons l ON l.id = w.lesson_id
		WHERE w.id = $1`, wordID).Scan(&classID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apperr.ErrNotFound
	}
	return classID, err
}
func (r *Repository) GetWord(ctx context.Context, wordID string) (*Word, error) {
	w, err := scanWord(r.db.QueryRow(ctx,
		`SELECT `+wordCols+` FROM words WHERE id = $1`, wordID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.ErrNotFound
	}
	return w, err
}

func (r *Repository) SetAudio(ctx context.Context, wordID string, url *string) (*Word, *string, error) {
	var w Word
	var old *string
	err := r.db.QueryRow(ctx, `
		WITH old AS (SELECT audio_url FROM words WHERE id = $1)
		UPDATE words SET audio_url = $2 WHERE id = $1
		RETURNING `+wordCols+`, (SELECT audio_url FROM old)`,
		wordID, url).Scan(&w.ID, &w.LessonID, &w.Hanzi, &w.Pinyin, &w.Meaning,
		&w.ExampleSentence, &w.AudioURL, &w.Position, &old)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, apperr.ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	return &w, old, nil
}

func (r *Repository) DeleteWord(ctx context.Context, wordID string) (*string, error) {
	var audio *string
	err := r.db.QueryRow(ctx,
		`DELETE FROM words WHERE id = $1 RETURNING audio_url`, wordID).Scan(&audio)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return audio, err
}
func (r *Repository) UpdateLesson(ctx context.Context, lessonID, title string, date time.Time) (*Lesson, error) {
	row := r.db.QueryRow(ctx, `
		WITH l AS (
			UPDATE lessons SET title = $2, lesson_date = $3 WHERE id = $1 RETURNING *
		)
		SELECT `+lessonCols+` FROM l`, lessonID, title, date)
	l, err := scanLesson(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.ErrNotFound
	}
	return l, err
}

// DeleteLesson removes the lesson and its words (cascade) and returns the
// audio URLs that no longer have a row, so files can be deleted too.
func (r *Repository) DeleteLesson(ctx context.Context, lessonID string) ([]string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	urls, err := lessonAudioURLs(ctx, tx, lessonID)
	if err != nil {
		return nil, err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM lessons WHERE id = $1`, lessonID)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, apperr.ErrNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return urls, nil
}

func lessonAudioURLs(ctx context.Context, tx pgx.Tx, lessonID string) ([]string, error) {
	rows, err := tx.Query(ctx,
		`SELECT audio_url FROM words WHERE lesson_id = $1 AND audio_url IS NOT NULL`, lessonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	urls := []string{}
	for rows.Next() {
		var url string
		if err := rows.Scan(&url); err != nil {
			return nil, err
		}
		urls = append(urls, url)
	}
	return urls, rows.Err()
}

func (r *Repository) UpdateWord(ctx context.Context, wordID string, w Word) (*Word, error) {
	row := r.db.QueryRow(ctx, `
		UPDATE words SET hanzi = $2, pinyin = $3, meaning = $4, example_sentence = $5
		WHERE id = $1
		RETURNING `+wordCols,
		wordID, w.Hanzi, w.Pinyin, w.Meaning, w.ExampleSentence)
	word, err := scanWord(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.ErrNotFound
	}
	return word, err
}

// ReorderWords sets position = 1..n in the given order. All ids must be the
// complete set of words in the lesson, with no duplicates.
func (r *Repository) ReorderWords(ctx context.Context, lessonID string, ids []string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var total int
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM words WHERE lesson_id = $1`, lessonID).Scan(&total); err != nil {
		return err
	}
	if total != len(ids) {
		return apperr.Validation("word_ids must contain every word in the lesson exactly once")
	}

	seen := make(map[string]bool, len(ids))
	for i, id := range ids {
		if seen[id] {
			return apperr.Validation("word_ids contains a duplicate")
		}
		seen[id] = true
		if _, err := uuid.Parse(id); err != nil {
			return apperr.Validation("word_ids contains an invalid id")
		}
		tag, err := tx.Exec(ctx,
			`UPDATE words SET position = $3 WHERE id = $1 AND lesson_id = $2`,
			id, lessonID, i+1)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return apperr.Validation("word_ids contains a word from another lesson")
		}
	}
	return tx.Commit(ctx)
}
