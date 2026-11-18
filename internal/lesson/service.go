package lesson

import (
	"bufio"
	"context"
	"io"
	"log"
	"net/http"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/yohannes/chines-backend/internal/auth"
	"github.com/yohannes/chines-backend/internal/class"
	"github.com/yohannes/chines-backend/internal/platform/apperr"
)

const (
	dateLayout    = "2006-01-02"
	MaxAudioBytes = 2 << 20 // 2 MB
	mediaPrefix   = "/media/audio/"
)

// What the file's first bytes say it is, mapped to the extension we save it with.
var audioTypes = map[string]string{
	"audio/mpeg":      ".mp3",
	"audio/wave":      ".wav",
	"application/ogg": ".ogg",
}

// FileStore is defined HERE, by the code that uses it. Any type with these
// two methods works: local disk now, S3 or R2 later.
type FileStore interface {
	Save(ctx context.Context, name string, r io.Reader) error
	Delete(ctx context.Context, name string) error
}

type Service struct {
	repo    *Repository
	classes *class.Repository
	store   FileStore
}

func NewService(repo *Repository, classes *class.Repository, store FileStore) *Service {
	return &Service{repo: repo, classes: classes, store: store}
}

// authorize asks the class module whether this user may view/manage the class.
func (s *Service) authorize(ctx context.Context, id auth.Identity, classID string, manage bool) error {
	ok, err := s.classes.Access(ctx, classID, id.UserID, id.Role, manage)
	if err != nil {
		return err
	}
	if !ok {
		return apperr.ErrNotFound
	}
	return nil
}

func (s *Service) CreateLesson(ctx context.Context, id auth.Identity, classID, title, date string) (*Lesson, error) {
	if err := s.authorize(ctx, id, classID, true); err != nil {
		return nil, err
	}
	title = strings.TrimSpace(title)
	if title == "" || utf8.RuneCountInString(title) > 150 {
		return nil, apperr.Validation("title is required (max 150 characters)")
	}
	d, err := time.Parse(dateLayout, date)
	if err != nil {
		return nil, apperr.Validation("lesson_date must look like 2026-11-14")
	}
	return s.repo.CreateLesson(ctx, classID, title, d)
}

func (s *Service) ListByClass(ctx context.Context, id auth.Identity, classID string) ([]Lesson, error) {
	if err := s.authorize(ctx, id, classID, false); err != nil {
		return nil, err
	}
	return s.repo.ListByClass(ctx, classID)
}

func (s *Service) ListByDate(ctx context.Context, id auth.Identity, date string) ([]Lesson, error) {
	d, err := time.Parse(dateLayout, date)
	if err != nil {
		return nil, apperr.Validation("date must look like 2026-11-14")
	}
	return s.repo.ListByDate(ctx, id.UserID, id.Role, d)
}

// Get returns one lesson together with its words.
func (s *Service) Get(ctx context.Context, id auth.Identity, lessonID string) (*Lesson, error) {
	l, err := s.repo.GetLesson(ctx, lessonID)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, id, l.ClassID, false); err != nil {
		return nil, err
	}
	l.Words, err = s.repo.ListWords(ctx, lessonID)
	if err != nil {
		return nil, err
	}
	return l, nil
}

type WordInput struct {
	Hanzi, Pinyin, Meaning, ExampleSentence string
}

func (s *Service) AddWord(ctx context.Context, id auth.Identity, lessonID string, in WordInput) (*Word, error) {
	l, err := s.repo.GetLesson(ctx, lessonID)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, id, l.ClassID, true); err != nil {
		return nil, err
	}

	w := Word{
		Hanzi:           strings.TrimSpace(in.Hanzi),
		Pinyin:          strings.TrimSpace(in.Pinyin),
		Meaning:         strings.TrimSpace(in.Meaning),
		ExampleSentence: strings.TrimSpace(in.ExampleSentence),
	}
	switch {
	case w.Hanzi == "" || utf8.RuneCountInString(w.Hanzi) > 50:
		return nil, apperr.Validation("hanzi is required (max 50 characters)")
	case w.Pinyin == "" || utf8.RuneCountInString(w.Pinyin) > 100:
		return nil, apperr.Validation("pinyin is required (max 100 characters)")
	case w.Meaning == "" || utf8.RuneCountInString(w.Meaning) > 200:
		return nil, apperr.Validation("meaning is required (max 200 characters)")
	case utf8.RuneCountInString(w.ExampleSentence) > 300:
		return nil, apperr.Validation("example_sentence is too long (max 300 characters)")
	}
	return s.repo.AddWord(ctx, lessonID, w)
}

// manageWord checks that the user may manage the class that owns this word.
func (s *Service) manageWord(ctx context.Context, id auth.Identity, wordID string) error {
	classID, err := s.repo.ClassIDOfWord(ctx, wordID)
	if err != nil {
		return err
	}
	return s.authorize(ctx, id, classID, true)
}

func (s *Service) SetAudio(ctx context.Context, id auth.Identity, wordID string, file io.Reader) (*Word, error) {
	if err := s.manageWord(ctx, id, wordID); err != nil {
		return nil, err
	}

	// Look at the first 512 bytes to find the real type.
	br := bufio.NewReader(file)
	head, err := br.Peek(512)
	if err != nil && err != io.EOF { // a short file is fine, it just gives fewer bytes
		return nil, err
	}
	ext, ok := audioTypes[http.DetectContentType(head)]
	if !ok {
		return nil, apperr.Validation("audio must be an MP3, WAV, or OGG file")
	}

	name := uuid.NewString() + ext
	if err := s.store.Save(ctx, name, br); err != nil {
		return nil, err
	}

	url := mediaPrefix + name
	word, old, err := s.repo.SetAudio(ctx, wordID, &url)
	if err != nil {
		_ = s.store.Delete(ctx, name) // don't leave a file that nothing points to
		return nil, err
	}
	s.removeFile(ctx, old)
	return word, nil
}

func (s *Service) ClearAudio(ctx context.Context, id auth.Identity, wordID string) (*Word, error) {
	if err := s.manageWord(ctx, id, wordID); err != nil {
		return nil, err
	}
	word, old, err := s.repo.SetAudio(ctx, wordID, nil)
	if err != nil {
		return nil, err
	}
	s.removeFile(ctx, old)
	return word, nil
}

func (s *Service) DeleteWord(ctx context.Context, id auth.Identity, wordID string) error {
	if err := s.manageWord(ctx, id, wordID); err != nil {
		return err
	}
	audio, err := s.repo.DeleteWord(ctx, wordID)
	if err != nil {
		return err
	}
	s.removeFile(ctx, audio)
	return nil
}

// removeFile deletes the file of an old audio path. A failure here is only
// logged: the database is already correct, and a leftover file is harmless.
func (s *Service) removeFile(ctx context.Context, url *string) {
	if url == nil {
		return
	}
	if err := s.store.Delete(ctx, path.Base(*url)); err != nil {
		log.Printf("could not delete audio file %s: %v", *url, err)
	}
}