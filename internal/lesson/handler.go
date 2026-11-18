package lesson

import (
	"net/http"

	"github.com/yohannes/chines-backend/internal/auth"
	"github.com/yohannes/chines-backend/internal/platform/apperr"
	"github.com/yohannes/chines-backend/internal/platform/httpx"
	"errors"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// POST /v1/classes/{id}/lessons
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	classID, ok := httpx.PathID(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		Title      string `json:"title"`
		LessonDate string `json:"lesson_date"`
	}
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	id, _ := auth.IdentityFrom(r.Context())
	l, err := h.svc.CreateLesson(r.Context(), id, classID, req.Title, req.LessonDate)
	if err != nil {
		apperr.Write(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, l)
}

// GET /v1/classes/{id}/lessons
func (h *Handler) ListByClass(w http.ResponseWriter, r *http.Request) {
	classID, ok := httpx.PathID(w, r, "id")
	if !ok {
		return
	}
	id, _ := auth.IdentityFrom(r.Context())
	lessons, err := h.svc.ListByClass(r.Context(), id, classID)
	if err != nil {
		apperr.Write(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, lessons)
}

// GET /v1/lessons?date=2026-11-14
// The app sends the phone's local date, so "today" is correct in Ethiopia (UTC+3).
func (h *Handler) ListByDate(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.IdentityFrom(r.Context())
	lessons, err := h.svc.ListByDate(r.Context(), id, r.URL.Query().Get("date"))
	if err != nil {
		apperr.Write(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, lessons)
}

// GET /v1/lessons/{id}
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	lessonID, ok := httpx.PathID(w, r, "id")
	if !ok {
		return
	}
	id, _ := auth.IdentityFrom(r.Context())
	l, err := h.svc.Get(r.Context(), id, lessonID)
	if err != nil {
		apperr.Write(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, l)
}

// POST /v1/lessons/{id}/words
func (h *Handler) AddWord(w http.ResponseWriter, r *http.Request) {
	lessonID, ok := httpx.PathID(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		Hanzi           string `json:"hanzi"`
		Pinyin          string `json:"pinyin"`
		Meaning         string `json:"meaning"`
		ExampleSentence string `json:"example_sentence"`
	}
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	id, _ := auth.IdentityFrom(r.Context())
	word, err := h.svc.AddWord(r.Context(), id, lessonID,
		WordInput{req.Hanzi, req.Pinyin, req.Meaning, req.ExampleSentence})
	if err != nil {
		apperr.Write(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, word)
}

// DELETE /v1/words/{id}
func (h *Handler) DeleteWord(w http.ResponseWriter, r *http.Request) {
	wordID, ok := httpx.PathID(w, r, "id")
	if !ok {
		return
	}
	id, _ := auth.IdentityFrom(r.Context())
	if err := h.svc.DeleteWord(r.Context(), id, wordID); err != nil {
		apperr.Write(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
// PUT /v1/words/{id}/audio   (multipart form, field name "audio")
func (h *Handler) UploadAudio(w http.ResponseWriter, r *http.Request) {
	wordID, ok := httpx.PathID(w, r, "id")
	if !ok {
		return
	}

	// Hard limit on how much we will even read (file + multipart overhead).
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
	word, err := h.svc.SetAudio(r.Context(), id, wordID, file)
	if err != nil {
		apperr.Write(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, word)
}

// DELETE /v1/words/{id}/audio
func (h *Handler) DeleteAudio(w http.ResponseWriter, r *http.Request) {
	wordID, ok := httpx.PathID(w, r, "id")
	if !ok {
		return
	}
	id, _ := auth.IdentityFrom(r.Context())
	word, err := h.svc.ClearAudio(r.Context(), id, wordID)
	if err != nil {
		apperr.Write(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, word)
}