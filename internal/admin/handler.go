package admin

import (
	"net/http"
	"strconv"

	"github.com/yohannes/chines-backend/internal/auth"
	"github.com/yohannes/chines-backend/internal/platform/apperr"
	"github.com/yohannes/chines-backend/internal/platform/httpx"
	"github.com/yohannes/chines-backend/internal/user"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// GET /v1/admin/overview
func (h *Handler) Overview(w http.ResponseWriter, r *http.Request) {
	o, err := h.svc.Overview(r.Context())
	if err != nil {
		apperr.Write(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, o)
}

// GET /v1/admin/users?role=&q=&page=&page_size=
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	page := parsePositive(r.URL.Query().Get("page"), 1)
	pageSize := parsePositive(r.URL.Query().Get("page_size"), 20)
	p, err := h.svc.ListUsers(r.Context(), r.URL.Query().Get("role"), r.URL.Query().Get("q"), page, pageSize)
	if err != nil {
		apperr.Write(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, p)
}

// POST /v1/admin/users
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	res, err := h.svc.CreateTeacher(r.Context(), CreateTeacherInput{req.Name, req.Email, req.Password})
	if err != nil {
		if err == user.ErrEmailTaken {
			httpx.Error(w, http.StatusConflict, "email already registered")
			return
		}
		apperr.Write(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, res)
}

// PATCH /v1/admin/users/{id}/role
func (h *Handler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	targetID, ok := httpx.PathID(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		Role string `json:"role"`
	}
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	id, _ := auth.IdentityFrom(r.Context())
	if err := h.svc.ChangeRole(r.Context(), id, targetID, req.Role); err != nil {
		apperr.Write(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /v1/admin/users/{id}/disable
func (h *Handler) Disable(w http.ResponseWriter, r *http.Request) {
	h.setDisabled(w, r, true)
}

// POST /v1/admin/users/{id}/enable
func (h *Handler) Enable(w http.ResponseWriter, r *http.Request) {
	h.setDisabled(w, r, false)
}

func (h *Handler) setDisabled(w http.ResponseWriter, r *http.Request, disabled bool) {
	targetID, ok := httpx.PathID(w, r, "id")
	if !ok {
		return
	}
	id, _ := auth.IdentityFrom(r.Context())
	if err := h.svc.SetDisabled(r.Context(), id, targetID, disabled); err != nil {
		apperr.Write(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parsePositive(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return fallback
	}
	return n
}
