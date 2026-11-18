package class

import "time"

type Class struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	TeacherID string    `json:"teacher_id"`
	CreatedAt time.Time `json:"created_at"`
}

type Member struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}