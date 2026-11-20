package class

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

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string `json:"name"`
		TeacherID string `json:"teacher_id"`
	}
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	id, _ := auth.IdentityFrom(r.Context())
	c, err := h.svc.Create(r.Context(), id, req.Name, req.TeacherID)
	if err != nil {
		apperr.Write(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, c)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.IdentityFrom(r.Context())
	classes, err := h.svc.List(r.Context(), id)
	if err != nil {
		apperr.Write(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, classes)
}

func (h *Handler) Members(w http.ResponseWriter, r *http.Request) {
	classID, ok := httpx.PathID(w, r, "id")
	if !ok {
		return
	}
	id, _ := auth.IdentityFrom(r.Context())
	members, err := h.svc.Members(r.Context(), id, classID)
	if err != nil {
		apperr.Write(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, members)
}

func (h *Handler) AddStudent(w http.ResponseWriter, r *http.Request) {
	classID, ok := httpx.PathID(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		Email string `json:"email"`
	}
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	id, _ := auth.IdentityFrom(r.Context())
	m, err := h.svc.AddStudent(r.Context(), id, classID, req.Email)
	if err != nil {
		apperr.Write(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, m)
}
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	classID, ok := httpx.PathID(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		Name      *string `json:"name"`
		TeacherID *string `json:"teacher_id"`
	}
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	id, _ := auth.IdentityFrom(r.Context())
	c, err := h.svc.Update(r.Context(), id, classID, req.Name, req.TeacherID)
	if err != nil {
		apperr.Write(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, c)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	classID, ok := httpx.PathID(w, r, "id")
	if !ok {
		return
	}
	id, _ := auth.IdentityFrom(r.Context())
	if err := h.svc.Delete(r.Context(), id, classID); err != nil {
		apperr.Write(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RemoveStudent(w http.ResponseWriter, r *http.Request) {
	classID, ok := httpx.PathID(w, r, "id")
	if !ok {
		return
	}
	studentID, ok := httpx.PathID(w, r, "studentId")
	if !ok {
		return
	}
	id, _ := auth.IdentityFrom(r.Context())
	if err := h.svc.RemoveStudent(r.Context(), id, classID, studentID); err != nil {
		apperr.Write(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
