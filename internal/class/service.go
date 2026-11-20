package class

import (
	"context"
	"errors"
	"log"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/yohannes/chines-backend/internal/auth"
	"github.com/yohannes/chines-backend/internal/platform/apperr"
	"github.com/yohannes/chines-backend/internal/user"
)

// FileStore is defined by the code that uses it (same shape as lesson.FileStore).
type FileStore interface {
	Delete(ctx context.Context, name string) error
}

type Service struct {
	repo  *Repository
	users *user.Repository
	store FileStore
}

func NewService(repo *Repository, users *user.Repository, store FileStore) *Service {
	return &Service{repo: repo, users: users, store: store}
}

func (s *Service) Create(ctx context.Context, id auth.Identity, name, teacherID string) (*Class, error) {
	name, err := validName(name)
	if err != nil {
		return nil, err
	}

	ownerID := id.UserID
	teacherID = strings.TrimSpace(teacherID)
	if teacherID != "" {
		if id.Role != user.RoleAdmin {
			return nil, apperr.Validation("only an admin can assign a class to another teacher")
		}
		if err := s.requireActiveTeacher(ctx, teacherID); err != nil {
			return nil, err
		}
		ownerID = teacherID
	}
	return s.repo.Create(ctx, name, ownerID)
}

func (s *Service) List(ctx context.Context, id auth.Identity) ([]Class, error) {
	return s.repo.ListFor(ctx, id.UserID, id.Role)
}

// Update renames a class and, for admins, moves it to another teacher. Both
// fields are optional so the same endpoint handles a rename and a reassignment.
func (s *Service) Update(ctx context.Context, id auth.Identity, classID string, name, teacherID *string) (*Class, error) {
	if err := s.mustManage(ctx, id, classID); err != nil {
		return nil, err
	}
	current, err := s.repo.Get(ctx, classID)
	if err != nil {
		return nil, err
	}

	newName := current.Name
	if name != nil {
		newName, err = validName(*name)
		if err != nil {
			return nil, err
		}
	}

	newTeacherID := current.TeacherID
	if teacherID != nil {
		if id.Role != user.RoleAdmin {
			return nil, apperr.Validation("only an admin can reassign a class")
		}
		newTeacherID = strings.TrimSpace(*teacherID)
		if newTeacherID == "" {
			return nil, apperr.Validation("teacher_id is required")
		}
		if err := s.requireActiveTeacher(ctx, newTeacherID); err != nil {
			return nil, err
		}
	}
	return s.repo.Update(ctx, classID, newName, newTeacherID)
}

func (s *Service) requireActiveTeacher(ctx context.Context, teacherID string) error {
	if _, err := uuid.Parse(teacherID); err != nil {
		return apperr.Validation("invalid teacher_id")
	}
	u, err := s.users.GetByID(ctx, teacherID)
	if errors.Is(err, user.ErrNotFound) {
		return apperr.Validation("no user with this id")
	}
	if err != nil {
		return err
	}
	if u.Role != user.RoleTeacher {
		return apperr.Validation("this user is not a teacher")
	}
	if u.DisabledAt != nil {
		return apperr.Validation("this teacher is disabled")
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, id auth.Identity, classID string) error {
	if err := s.mustManage(ctx, id, classID); err != nil {
		return err
	}
	audio, err := s.repo.Delete(ctx, classID)
	if err != nil {
		return err
	}
	for _, url := range audio {
		if err := s.store.Delete(ctx, path.Base(url)); err != nil {
			log.Printf("could not delete audio file %s: %v", url, err)
		}
	}
	return nil
}

func (s *Service) Members(ctx context.Context, id auth.Identity, classID string) ([]Member, error) {
	if err := s.mustManage(ctx, id, classID); err != nil {
		return nil, err
	}
	return s.repo.ListMembers(ctx, classID)
}

func (s *Service) AddStudent(ctx context.Context, id auth.Identity, classID, email string) (*Member, error) {
	if err := s.mustManage(ctx, id, classID); err != nil {
		return nil, err
	}

	email = strings.ToLower(strings.TrimSpace(email))
	u, err := s.users.GetByEmail(ctx, email)
	if errors.Is(err, user.ErrNotFound) {
		return nil, apperr.Validation("no registered user with this email")
	}
	if err != nil {
		return nil, err
	}
	if u.Role != user.RoleStudent {
		return nil, apperr.Validation("this user is not a student")
	}
	if u.DisabledAt != nil {
		return nil, apperr.Validation("this student is disabled")
	}

	if err := s.repo.AddMember(ctx, classID, u.ID); err != nil {
		return nil, err
	}
	return &Member{ID: u.ID, Name: u.Name, Email: u.Email}, nil
}

func (s *Service) RemoveStudent(ctx context.Context, id auth.Identity, classID, studentID string) error {
	if err := s.mustManage(ctx, id, classID); err != nil {
		return err
	}
	return s.repo.RemoveMember(ctx, classID, studentID)
}

func (s *Service) mustManage(ctx context.Context, id auth.Identity, classID string) error {
	ok, err := s.repo.Access(ctx, classID, id.UserID, id.Role, true)
	if err != nil {
		return err
	}
	if !ok {
		return apperr.ErrNotFound
	}
	return nil
}

func validName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 100 {
		return "", apperr.Validation("name is required (max 100 characters)")
	}
	return name, nil
}
