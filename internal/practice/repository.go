package practice

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yohannes/chines-backend/internal/lesson"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Item is a word to practice. The embedded lesson.Word fields appear
// directly in the JSON, next to is_new.
type Item struct {
	lesson.Word
	IsNew bool `json:"is_new"`
}

type Summary struct {
	DueCount int `json:"due_count"` // words to review
	NewCount int `json:"new_count"` // words never practiced
}

type Progress struct {
	WordID       string    `json:"word_id"`
	EaseFactor   float64   `json:"ease_factor"`
	IntervalDays int       `json:"interval_days"`
	Repetitions  int       `json:"repetitions"`
	NextReviewAt time.Time `json:"next_review_at"`
}

// The FROM/WHERE part is shared by the list query and the count query.
// $1 = student id, $2 = today's date (sent by the phone).
const dueFrom = `
	FROM class_members m
	JOIN lessons l ON l.class_id = m.class_id AND l.lesson_date <= $2
	JOIN words w ON w.lesson_id = l.id
	LEFT JOIN word_progress p ON p.word_id = w.id AND p.student_id = m.student_id
	WHERE m.student_id = $1
	  AND (p.word_id IS NULL OR p.next_review_at <= now())`

func (r *Repository) Due(ctx context.Context, studentID string, today time.Time, limit int) ([]Item, error) {
	rows, err := r.db.Query(ctx, `
		SELECT w.id::text, w.lesson_id::text, w.hanzi, w.pinyin, w.meaning,
		       w.example_sentence, w.audio_url, w.position, (p.word_id IS NULL)`+dueFrom+`
		ORDER BY (p.word_id IS NULL), p.next_review_at, l.lesson_date DESC, w.position
		LIMIT $3`, studentID, today, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []Item{}
	for rows.Next() {
		var it Item
		err := rows.Scan(&it.ID, &it.LessonID, &it.Hanzi, &it.Pinyin, &it.Meaning,
			&it.ExampleSentence, &it.AudioURL, &it.Position, &it.IsNew)
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (r *Repository) Summary(ctx context.Context, studentID string, today time.Time) (*Summary, error) {
	var s Summary
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FILTER (WHERE p.word_id IS NOT NULL),
		       COUNT(*) FILTER (WHERE p.word_id IS NULL)`+dueFrom,
		studentID, today).Scan(&s.DueCount, &s.NewCount)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// Answer records one answer and reschedules the word, all in ONE transaction:
// either everything is saved, or nothing is.
func (r *Repository) Answer(ctx context.Context, studentID, wordID, typ string, quality int) (*Progress, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) // does nothing if Commit already succeeded

	// 1. Make sure a progress row exists (first time this student sees the word).
	if _, err := tx.Exec(ctx,
		`INSERT INTO word_progress (student_id, word_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		studentID, wordID); err != nil {
		return nil, err
	}

	// 2. Read the current state and LOCK the row, so two answers sent at the
	//    same moment can't overwrite each other.
	var cur State
	err = tx.QueryRow(ctx, `
		SELECT ease_factor, interval_days, repetitions
		FROM word_progress
		WHERE student_id = $1 AND word_id = $2
		FOR UPDATE`, studentID, wordID).Scan(&cur.EaseFactor, &cur.IntervalDays, &cur.Repetitions)
	if err != nil {
		return nil, err
	}

	// 3. Run SM-2.
	next := Next(cur, quality, time.Now())

	// 4. Save the new state and the history row.
	if _, err := tx.Exec(ctx, `
		UPDATE word_progress
		SET ease_factor = $3, interval_days = $4, repetitions = $5,
		    next_review_at = $6, last_reviewed_at = now()
		WHERE student_id = $1 AND word_id = $2`,
		studentID, wordID, next.EaseFactor, next.IntervalDays, next.Repetitions, next.NextReviewAt); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO practice_attempts (student_id, word_id, type, quality, is_correct)
		VALUES ($1, $2, $3, $4, $5)`,
		studentID, wordID, typ, quality, quality >= 3); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &Progress{
		WordID:       wordID,
		EaseFactor:   next.EaseFactor,
		IntervalDays: next.IntervalDays,
		Repetitions:  next.Repetitions,
		NextReviewAt: next.NextReviewAt,
	}, nil
}