package user

import (
	"errors"
	"time"
)

type Role string

const (
	RoleStudent Role = "student"
	RoleTeacher Role = "teacher"
	RoleAdmin   Role = "admin"
)

func (r Role) Valid() bool {
	switch r {
	case RoleStudent, RoleTeacher, RoleAdmin:
		return true
	default:
		return false
	}
}

type User struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"` // never sent to clients
	Role         Role       `json:"role"`
	DisabledAt   *time.Time `json:"disabled_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

var (
	ErrNotFound   = errors.New("user not found")
	ErrEmailTaken = errors.New("email already registered")
)
