package class

import (
	"context"

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

// classCols is shared by every class query so the dashboard always receives the
// teacher name and student count it renders, without a follow-up request.
const classCols = `c.id::text, c.name, c.teacher_id::text, COALESCE(t.name, ''), COALESCE(t.role::text, ''),
	(SELECT COUNT(*)::int FROM class_members m WHERE m.class_id = c.id), c.created_at`

func scanClass(row pgx.Row) (*Class, error) {
	var c Class
	if err := row.Scan(&c.ID, &c.Name, &c.TeacherID, &c.TeacherName,
		&c.TeacherRole, &c.StudentCount, &c.CreatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *Repository) Create(ctx context.Context, name, teacherID string) (*Class, error) {
	row := r.db.QueryRow(ctx, `
		WITH c AS (
			INSERT INTO classes (name, teacher_id) VALUES ($1, $2) RETURNING *
		)
		SELECT `+classCols+` FROM c JOIN users t ON t.id = c.teacher_id`,
		name, teacherID)
	return scanClass(row)
}

// ListFor returns the classes this user is allowed to see.
func (r *Repository) ListFor(ctx context.Context, userID string, role user.Role) ([]Class, error) {
	const base = `SELECT ` + classCols + ` FROM classes c JOIN users t ON t.id = c.teacher_id `

	var rows pgx.Rows
	var err error
	switch role {
	case user.RoleAdmin:
		rows, err = r.db.Query(ctx, base+`ORDER BY c.created_at DESC`)
	case user.RoleTeacher:
		rows, err = r.db.Query(ctx, base+`WHERE c.teacher_id = $1 ORDER BY c.created_at DESC`, userID)
	default: // student
		rows, err = r.db.Query(ctx, base+
			`JOIN class_members m ON m.class_id = c.id
			 WHERE m.student_id = $1 ORDER BY c.created_at DESC`, userID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	classes := []Class{} // empty slice, so JSON shows [] and not null
	for rows.Next() {
		c, err := scanClass(rows)
		if err != nil {
			return nil, err
		}
		classes = append(classes, *c)
	}
	return classes, rows.Err()
}

// Access says whether the user may view (manage=false) or manage (manage=true) the class.
// Other modules (lessons) call this too, so the rule lives in ONE place.
func (r *Repository) Access(ctx context.Context, classID, userID string, role user.Role, manage bool) (bool, error) {
	args := []any{classID}
	var q string

	switch {
	case role == user.RoleAdmin:
		q = `SELECT EXISTS (SELECT 1 FROM classes WHERE id = $1)`
	case role == user.RoleTeacher:
		q = `SELECT EXISTS (SELECT 1 FROM classes WHERE id = $1 AND teacher_id = $2)`
		args = append(args, userID)
	case role == user.RoleStudent && !manage:
		q = `SELECT EXISTS (SELECT 1 FROM class_members WHERE class_id = $1 AND student_id = $2)`
		args = append(args, userID)
	default:
		return false, nil
	}

	var ok bool
	err := r.db.QueryRow(ctx, q, args...).Scan(&ok)
	return ok, err
}

func (r *Repository) AddMember(ctx context.Context, classID, studentID string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO class_members (class_id, student_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		classID, studentID)
	return err
}

func (r *Repository) ListMembers(ctx context.Context, classID string) ([]Member, error) {
	rows, err := r.db.Query(ctx, `
		SELECT u.id::text, u.name, u.email
		FROM class_members m
		JOIN users u ON u.id = m.student_id
		WHERE m.class_id = $1
		ORDER BY u.name`, classID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := []Member{}
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.ID, &m.Name, &m.Email); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}
func (r *Repository) Get(ctx context.Context, classID string) (*Class, error) {
	row := r.db.QueryRow(ctx,
		`SELECT `+classCols+` FROM classes c JOIN users t ON t.id = c.teacher_id
		 WHERE c.id = $1`, classID)
	c, err := scanClass(row)
	if err == pgx.ErrNoRows {
		return nil, apperr.ErrNotFound
	}
	return c, err
}

func (r *Repository) Update(ctx context.Context, classID, name, teacherID string) (*Class, error) {
	row := r.db.QueryRow(ctx, `
		WITH c AS (
			UPDATE classes SET name = $2, teacher_id = $3 WHERE id = $1 RETURNING *
		)
		SELECT `+classCols+` FROM c JOIN users t ON t.id = c.teacher_id`,
		classID, name, teacherID)
	c, err := scanClass(row)
	if err == pgx.ErrNoRows {
		return nil, apperr.ErrNotFound
	}
	return c, err
}

// Delete removes the class (lessons, words, members cascade in the DB) and
// returns every audio URL that belonged to it so the caller can remove files.
func (r *Repository) Delete(ctx context.Context, classID string) ([]string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT w.audio_url
		FROM words w JOIN lessons l ON l.id = w.lesson_id
		WHERE l.class_id = $1 AND w.audio_url IS NOT NULL`, classID)
	if err != nil {
		return nil, err
	}
	urls := []string{}
	for rows.Next() {
		var url string
		if err := rows.Scan(&url); err != nil {
			rows.Close()
			return nil, err
		}
		urls = append(urls, url)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	tag, err := tx.Exec(ctx, `DELETE FROM classes WHERE id = $1`, classID)
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

func (r *Repository) RemoveMember(ctx context.Context, classID, studentID string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM class_members WHERE class_id = $1 AND student_id = $2`,
		classID, studentID)
	return err
}
