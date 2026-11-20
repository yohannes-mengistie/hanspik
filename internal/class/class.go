package class

import "time"

type Class struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	TeacherID    string    `json:"teacher_id"`
	TeacherName  string    `json:"teacher_name"`
	TeacherRole  string    `json:"teacher_role"`
	StudentCount int       `json:"student_count"`
	CreatedAt    time.Time `json:"created_at"`
}

type Member struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}
