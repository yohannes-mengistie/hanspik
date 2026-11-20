package httpx

import (
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"net/http"
)

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func Error(w http.ResponseWriter, status int, msg string) {
	JSON(w, status, map[string]string{"error": msg})
}

// Decode reads a JSON body, limited to 1 MB, rejecting unknown fields.
func Decode(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

// PathID reads a URL parameter and checks that it is a valid UUID.
// A malformed id would otherwise make PostgreSQL return an error (500).
func PathID(w http.ResponseWriter, r *http.Request, name string) (string, bool) {
	id := chi.URLParam(r, name)
	if _, err := uuid.Parse(id); err != nil {
		Error(w, http.StatusNotFound, "not found")
		return "", false
	}
	return id, true
}
