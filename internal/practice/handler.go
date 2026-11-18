package practice

import (
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/yohannes/chines-backend/internal/auth"
	"github.com/yohannes/chines-backend/internal/platform/apperr"
	"github.com/yohannes/chines-backend/internal/platform/httpx"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// GET /v1/practice/due?date=2026-11-17&limit=20
func (h *Handler) Due(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 50 {
			apperr.Write(w, apperr.Validation("limit must be between 1 and 50"))
			return
		}
		limit = n
	}
	id, _ := auth.IdentityFrom(r.Context())
	items, err := h.svc.Due(r.Context(), id, r.URL.Query().Get("date"), limit)
	if err != nil {
		apperr.Write(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

// GET /v1/practice/summary?date=2026-11-17
func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.IdentityFrom(r.Context())
	s, err := h.svc.Summary(r.Context(), id, r.URL.Query().Get("date"))
	if err != nil {
		apperr.Write(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, s)
}

// POST /v1/practice/answer
func (h *Handler) Answer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WordID  string `json:"word_id"`
		Type    string `json:"type"`
		Quality *int   `json:"quality"` // pointer: lets us tell "missing" from 0
	}
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if _, err := uuid.Parse(req.WordID); err != nil {
		apperr.Write(w, apperr.Validation("word_id is not valid"))
		return
	}
	if req.Quality == nil {
		apperr.Write(w, apperr.Validation("quality is required"))
		return
	}

	id, _ := auth.IdentityFrom(r.Context())
	p, err := h.svc.Answer(r.Context(), id, req.WordID, req.Type, *req.Quality)
	if err != nil {
		apperr.Write(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, p)
}