package apperr

import (
	"errors"
	"log"
	"net/http"
	"github.com/yohannes/chines-backend/internal/platform/httpx"
)

var ErrNotFound = errors.New("not found")

type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

func Validation(msg string) error { return &ValidationError{Msg: msg} }

// Write converts an error into the right HTTP response.
func Write(w http.ResponseWriter, err error) {
	var ve *ValidationError
	switch {
	case errors.As(err, &ve):
		httpx.Error(w, http.StatusBadRequest, ve.Msg)
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "not found")
	default:
		log.Printf("internal error: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "internal server error")
	}
}