package teacher

import (
	"context"
	"errors"
	"time"

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

func (r *Repository) Overview(ctx context.Context, userID string, role user.Role) (*Overview, error) {
	var o Overview
	var err error
	var classes, students, active int64

	if role == user.RoleAdmin {
		err = r.db.QueryRow(ctx, `
			SELECT
			  (SELECT COUNT(*) FROM classes),
			  (SELECT COUNT(DISTINCT student_id) FROM class_members),
			  (SELECT COUNT(DISTINCT a.student_id) FROM practice_attempts a
			    WHERE a.created_at >= now() - interval '7 days')`).
			Scan(&classes, &students, &active)
	} else {
		err = r.db.QueryRow(ctx, `
			WITH mine AS (SELECT id FROM classes WHERE teacher_id = $1)
			SELECT
			  (SELECT COUNT(*) FROM mine),
			  (SELECT COUNT(DISTINCT m.student_id) FROM class_members m
			    WHERE m.class_id IN (SELECT id FROM mine)),
			  (SELECT COUNT(DISTINCT a.student_id)
			     FROM practice_attempts a
			     JOIN words w ON w.id = a.word_id
			     JOIN lessons l ON l.id = w.lesson_id
			     WHERE l.class_id IN (SELECT id FROM mine)
			       AND a.created_at >= now() - interval '7 days')`, userID).
			Scan(&classes, &students, &active)
	}
	if err != nil {
		return nil, err
	}
	o.Classes, o.Students, o.ActiveStudents7d = int(classes), int(students), int(active)
	return &o, nil
}

// ClassProgress aggregates per student in one page and never queries per row.
func (r *Repository) ClassProgress(ctx context.Context, classID string) ([]StudentProgress, error) {
	rows, err := r.db.Query(ctx, `
		SELECT u.id::text, u.name, u.email,
		       COALESCE(wp.practiced, 0),
		       COALESCE(ap.attempts, 0),
		       COALESCE(ap.correct, 0),
		       COALESCE(wp.due, 0),
		       ap.last_active,
		       pr.avg_score
		FROM class_members m
		JOIN users u ON u.id = m.student_id
		LEFT JOIN LATERAL (
			SELECT COUNT(*) AS practiced,
			       COUNT(*) FILTER (WHERE p.next_review_at <= now()) AS due
			FROM word_progress p
			JOIN words w ON w.id = p.word_id
			JOIN lessons l ON l.id = w.lesson_id
			WHERE p.student_id = u.id AND l.class_id = $1
		) wp ON true
		LEFT JOIN LATERAL (
			SELECT COUNT(*) AS attempts,
			       COUNT(*) FILTER (WHERE a.is_correct) AS correct,
			       MAX(a.created_at) AS last_active
			FROM practice_attempts a
			JOIN words w ON w.id = a.word_id
			JOIN lessons l ON l.id = w.lesson_id
			WHERE a.student_id = u.id AND l.class_id = $1
		) ap ON true
		LEFT JOIN LATERAL (
			SELECT AVG(pa.overall_score) AS avg_score
			FROM pronunciation_attempts pa
			JOIN words w ON w.id = pa.word_id
			JOIN lessons l ON l.id = w.lesson_id
			WHERE pa.student_id = u.id AND l.class_id = $1
		) pr ON true
		WHERE m.class_id = $1
		ORDER BY u.name`, classID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	students := []StudentProgress{}
	index := map[string]int{}
	for rows.Next() {
		var s StudentProgress
		var practiced, attempts, correct, due int64
		if err := rows.Scan(&s.StudentID, &s.Name, &s.Email,
			&practiced, &attempts, &correct, &due, &s.LastActive,
			&s.AveragePronunciationScore); err != nil {
			return nil, err
		}
		s.WordsPracticed = int(practiced)
		s.DueCount = int(due)
		s.Accuracy = percent(correct, attempts)
		index[s.StudentID] = len(students)
		students = append(students, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	days, err := r.studyDays(ctx, classID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	for i := range students {
		students[i].Streak = streakFromDays(days[students[i].StudentID], now)
	}
	return students, nil
}

// studyDays returns, per student, the set of UTC dates on which they answered
// a question in this class. One query, then the streak is computed in Go.
func (r *Repository) studyDays(ctx context.Context, classID string) (map[string]map[string]bool, error) {
	rows, err := r.db.Query(ctx, `
		SELECT a.student_id::text, date(a.created_at) AS day
		FROM practice_attempts a
		JOIN words w ON w.id = a.word_id
		JOIN lessons l ON l.id = w.lesson_id
		WHERE l.class_id = $1 AND a.created_at >= now() - interval '400 days'
		GROUP BY a.student_id, day`, classID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]map[string]bool{}
	for rows.Next() {
		var studentID string
		var day time.Time
		if err := rows.Scan(&studentID, &day); err != nil {
			return nil, err
		}
		if out[studentID] == nil {
			out[studentID] = map[string]bool{}
		}
		out[studentID][day.Format("2006-01-02")] = true
	}
	return out, rows.Err()
}

func (r *Repository) StudentDetail(ctx context.Context, classID, studentID string) (*StudentDetail, error) {
	var d StudentDetail
	err := r.db.QueryRow(ctx, `
		SELECT u.id::text, u.name, u.email
		FROM class_members m JOIN users u ON u.id = m.student_id
		WHERE m.class_id = $1 AND m.student_id = $2`, classID, studentID).
		Scan(&d.StudentID, &d.Name, &d.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	d.Lessons, err = r.lessonAccuracy(ctx, classID, studentID)
	if err != nil {
		return nil, err
	}
	d.HardestWords, err = r.hardestWords(ctx, classID, studentID)
	if err != nil {
		return nil, err
	}
	d.RecentAttempts, err = r.recentAttempts(ctx, classID, studentID)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *Repository) lessonAccuracy(ctx context.Context, classID, studentID string) ([]LessonAccuracy, error) {
	rows, err := r.db.Query(ctx, `
		SELECT l.id::text, l.title,
		       COUNT(a.id),
		       COUNT(a.id) FILTER (WHERE a.is_correct)
		FROM lessons l
		LEFT JOIN words w ON w.lesson_id = l.id
		LEFT JOIN practice_attempts a ON a.word_id = w.id AND a.student_id = $2
		WHERE l.class_id = $1
		GROUP BY l.id, l.title, l.lesson_date
		ORDER BY l.lesson_date, l.title`, classID, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []LessonAccuracy{}
	for rows.Next() {
		var l LessonAccuracy
		var attempts, correct int64
		if err := rows.Scan(&l.LessonID, &l.Title, &attempts, &correct); err != nil {
			return nil, err
		}
		l.Attempts, l.Correct = int(attempts), int(correct)
		l.Accuracy = percent(correct, attempts)
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r *Repository) hardestWords(ctx context.Context, classID, studentID string) ([]WordStat, error) {
	rows, err := r.db.Query(ctx, `
		SELECT w.id::text, w.hanzi, w.pinyin, w.meaning,
		       COUNT(a.id) AS attempts,
		       COUNT(a.id) FILTER (WHERE a.is_correct) AS correct
		FROM words w JOIN lessons l ON l.id = w.lesson_id
		LEFT JOIN practice_attempts a ON a.word_id = w.id AND a.student_id = $2
		WHERE l.class_id = $1
		GROUP BY w.id, w.hanzi, w.pinyin, w.meaning
		HAVING COUNT(a.id) > 0
		ORDER BY (COUNT(a.id) FILTER (WHERE a.is_correct)::float / COUNT(a.id)) ASC,
		         COUNT(a.id) DESC
		LIMIT 10`, classID, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []WordStat{}
	for rows.Next() {
		var w WordStat
		var attempts, correct int64
		if err := rows.Scan(&w.WordID, &w.Hanzi, &w.Pinyin, &w.Meaning, &attempts, &correct); err != nil {
			return nil, err
		}
		w.Attempts, w.Correct = int(attempts), int(correct)
		w.Accuracy = percent(correct, attempts)
		out = append(out, w)
	}
	return out, rows.Err()
}

func (r *Repository) recentAttempts(ctx context.Context, classID, studentID string) ([]Attempt, error) {
	rows, err := r.db.Query(ctx, `
		SELECT a.word_id::text, w.hanzi, a.type, a.quality, a.is_correct, a.created_at
		FROM practice_attempts a
		JOIN words w ON w.id = a.word_id
		JOIN lessons l ON l.id = w.lesson_id
		WHERE a.student_id = $2 AND l.class_id = $1
		ORDER BY a.created_at DESC
		LIMIT 20`, classID, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Attempt{}
	for rows.Next() {
		var a Attempt
		if err := rows.Scan(&a.WordID, &a.Hanzi, &a.Type, &a.Quality, &a.IsCorrect, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
