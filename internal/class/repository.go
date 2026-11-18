package class

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yohannes/chines-backend/internal/user"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, name, teacherID string) (*Class, error) {
	c := &Class{Name: name, TeacherID: teacherID}
	err := r.db.QueryRow(ctx,
		`INSERT INTO classes (name, teacher_id) VALUES ($1, $2) RETURNING id::text, created_at`,
		name, teacherID).Scan(&c.ID, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// ListFor returns the classes this user is allowed to see.
func (r *Repository) ListFor(ctx context.Context, userID string, role user.Role) ([]Class, error) {
	const base = `SELECT c.id::text, c.name, c.teacher_id::text, c.created_at FROM classes c `

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
		var c Class
		if err := rows.Scan(&c.ID, &c.Name, &c.TeacherID, &c.CreatedAt); err != nil {
			return nil, err
		}
		classes = append(classes, c)
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