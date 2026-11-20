package pronunciation

import (
	"errors"
	"io"
	"net/http"

	"github.com/google/uuid"

	"github.com/yohannes/chines-backend/internal/auth"
	"github.com/yohannes/chines-backend/internal/platform/apperr"
	"github.com/yohannes/chines-backend/internal/platform/httpx"
)

var contentType = map[string]string{
	".wav": "audio/wav",
	".mp3": "audio/mpeg",
	".ogg": "audio/ogg",
}

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// POST /v1/pronunciation/{wordId}
func (h *Handler) Score(w http.ResponseWriter, r *http.Request) {
	wordID, ok := httpx.PathID(w, r, "wordId")
	if !ok {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, MaxAudioBytes+(1<<20))
	file, header, err := r.FormFile("audio")
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			httpx.Error(w, http.StatusRequestEntityTooLarge, "file too large (max 2 MB)")
			return
		}
		httpx.Error(w, http.StatusBadRequest, `send the file as multipart form field "audio"`)
		return
	}
	defer file.Close()
	if header.Size > MaxAudioBytes {
		httpx.Error(w, http.StatusRequestEntityTooLarge, "file too large (max 2 MB)")
		return
	}

	id, _ := auth.IdentityFrom(r.Context())
	attempt, err := h.svc.Score(r.Context(), id, wordID, file)
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, attempt)
}

// GET /v1/pronunciation/history?wordId=...
func (h *Handler) History(w http.ResponseWriter, r *http.Request) {
	wordID := r.URL.Query().Get("wordId")
	if _, err := uuid.Parse(wordID); err != nil {
		httpx.Error(w, http.StatusBadRequest, "wordId must be a UUID")
		return
	}
	id, _ := auth.IdentityFrom(r.Context())
	attempts, err := h.svc.History(r.Context(), id, wordID, 20)
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": attempts})
}

// GET /v1/pronunciation/{id}/audio
func (h *Handler) Audio(w http.ResponseWriter, r *http.Request) {
	attemptID, ok := httpx.PathID(w, r, "id")
	if !ok {
		return
	}
	id, _ := auth.IdentityFrom(r.Context())
	file, ext, err := h.svc.OpenAudio(r.Context(), id, attemptID)
	if err != nil {
		writeError(w, err)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", contentType[ext])
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, file)
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrRateLimited):
		httpx.Error(w, http.StatusTooManyRequests, "too many attempts, try again in a minute")
	case errors.Is(err, ErrUnsupportedAudio):
		httpx.Error(w, http.StatusUnsupportedMediaType, "send a WAV, MP3, or OGG recording")
	case errors.Is(err, ErrProvider):
		httpx.Error(w, http.StatusBadGateway, "pronunciation scoring is temporarily unavailable")
	default:
		var ve *apperr.ValidationError
		if errors.As(err, &ve) {
			httpx.Error(w, http.StatusBadRequest, ve.Msg)
			return
		}
		apperr.Write(w, err)
	}
}
