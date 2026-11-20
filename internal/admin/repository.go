package admin

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Overview is one round trip for the admin landing page. UnassignedClasses
// counts classes still owned by an admin (for example, created before a teacher
// was chosen); the dashboard uses it to prompt for a teacher assignment.
func (r *Repository) Overview(ctx context.Context) (*Overview, error) {
	var o Overview
	err := r.db.QueryRow(ctx, `
		SELECT
		  (SELECT COUNT(*) FROM classes),
		  (SELECT COUNT(*) FROM users WHERE role = 'student'),
		  (SELECT COUNT(*) FROM users WHERE role = 'teacher'),
		  (SELECT COUNT(*) FROM users WHERE role = 'admin'),
		  (SELECT COUNT(DISTINCT a.student_id)
		     FROM practice_attempts a
		     JOIN users u ON u.id = a.student_id
		    WHERE u.role = 'student'
		      AND a.created_at >= now() - interval '7 days'),
		  (SELECT COUNT(*)
		     FROM classes c
		     JOIN users u ON u.id = c.teacher_id
		    WHERE u.role <> 'teacher' OR u.disabled_at IS NOT NULL)`).
		Scan(&o.Classes, &o.Students, &o.Teachers, &o.Admins,
			&o.ActiveStudents7d, &o.UnassignedClasses)
	if err != nil {
		return nil, err
	}
	return &o, nil
}
