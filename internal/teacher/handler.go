package teacher

import (
	"net/http"

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

// GET /v1/teacher/overview
func (h *Handler) Overview(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.IdentityFrom(r.Context())
	o, err := h.svc.Overview(r.Context(), id)
	if err != nil {
		apperr.Write(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, o)
}

// GET /v1/teacher/classes/{id}/progress
func (h *Handler) Progress(w http.ResponseWriter, r *http.Request) {
	classID, ok := httpx.PathID(w, r, "id")
	if !ok {
		return
	}
	id, _ := auth.IdentityFrom(r.Context())
	p, err := h.svc.ClassProgress(r.Context(), id, classID)
	if err != nil {
		apperr.Write(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, p)
}

// GET /v1/teacher/classes/{id}/students/{studentId}
func (h *Handler) Student(w http.ResponseWriter, r *http.Request) {
	classID, ok := httpx.PathID(w, r, "id")
	if !ok {
		return
	}
	studentID, ok := httpx.PathID(w, r, "studentId")
	if !ok {
		return
	}
	id, _ := auth.IdentityFrom(r.Context())
	d, err := h.svc.StudentDetail(r.Context(), id, classID, studentID)
	if err != nil {
		apperr.Write(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, d)
}
