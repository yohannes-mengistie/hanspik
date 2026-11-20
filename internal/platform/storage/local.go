package storage

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"github.com/go-chi/chi/v5"
)

// Local stores files in one folder on disk.
type Local struct {
	dir string
}

func NewLocal(dir string) (*Local, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Local{dir: dir}, nil
}

// Save writes to a temporary file first and renames it at the end, so a
// half-written file (connection lost, disk full) is never visible to students.
func (l *Local) Save(ctx context.Context, name string, r io.Reader) error {
	tmp, err := os.CreateTemp(l.dir, "tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // after a successful rename this finds nothing and does nothing

	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(l.dir, filepath.Base(name)))
}

// Open returns a private file for authenticated streaming. Callers must
// validate the name themselves; name is reduced to its base to block traversal.
func (l *Local) Open(ctx context.Context, name string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(l.dir, filepath.Base(name)))
}

func (l *Local) Delete(ctx context.Context, name string) error {
	err := os.Remove(filepath.Join(l.dir, filepath.Base(name)))
	if errors.Is(err, os.ErrNotExist) {
		return nil // already gone, which is what we wanted
	}
	return err
}

// Only names we generate ourselves are served. This also blocks "../" tricks.
var audioName = regexp.MustCompile(`^[0-9a-f-]{36}\.(mp3|wav|ogg)$`)

var contentTypes = map[string]string{
	".mp3": "audio/mpeg",
	".wav": "audio/wav",
	".ogg": "audio/ogg",
}

// Serve handles GET /media/audio/{name}.
// http.ServeFile supports Range requests, which phones use to seek inside audio.
func (l *Local) Serve(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !audioName.MatchString(name) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", contentTypes[filepath.Ext(name)])
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// The name is unique and never changes content, so clients can cache it forever.
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, filepath.Join(l.dir, name))
}
