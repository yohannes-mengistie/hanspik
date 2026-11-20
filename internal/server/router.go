package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yohannes/chines-backend/internal/admin"
	"github.com/yohannes/chines-backend/internal/auth"
	"github.com/yohannes/chines-backend/internal/class"
	"github.com/yohannes/chines-backend/internal/lesson"
	"github.com/yohannes/chines-backend/internal/practice"
	"github.com/yohannes/chines-backend/internal/pronunciation"
	"github.com/yohannes/chines-backend/internal/teacher"
	"github.com/yohannes/chines-backend/internal/user"
)

type Handlers struct {
	Auth          *auth.Handler
	Class         *class.Handler
	Lesson        *lesson.Handler
	Practice      *practice.Handler
	Pronunciation *pronunciation.Handler
	Teacher       *teacher.Handler
	Admin         *admin.Handler
	Media         http.HandlerFunc
}

func NewRouter(db *pgxpool.Pool, tokens *auth.TokenManager, h Handlers) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/health", healthHandler(db))
	r.Get("/media/audio/{name}", h.Media)

	staff := auth.RequireRole(user.RoleTeacher, user.RoleAdmin)
	adminOnly := auth.RequireRole(user.RoleAdmin)

	// Practice (students only)
	r.Route("/practice", func(r chi.Router) {
		r.Use(auth.RequireRole(user.RoleStudent))
		r.Get("/due", h.Practice.Due)
		r.Get("/summary", h.Practice.Summary)
		r.Post("/answer", h.Practice.Answer)
	})

	r.Route("/v1", func(r chi.Router) {
		// Public
		r.Route("/auth", h.Auth.Routes)

		// Logged-in users
		r.Group(func(r chi.Router) {
			r.Use(auth.Authenticate(tokens))

			r.Get("/me", h.Auth.Me)

			// Classes
			r.Get("/classes", h.Class.List)
			r.With(staff).Post("/classes", h.Class.Create)
			r.Route("/classes/{id}", func(r chi.Router) {
				r.With(staff).Patch("/", h.Class.Update)
				r.With(staff).Delete("/", h.Class.Delete)
				r.With(staff).Get("/members", h.Class.Members)
				r.With(staff).Post("/members", h.Class.AddStudent)
				r.With(staff).Delete("/members/{studentId}", h.Class.RemoveStudent)
				r.With(staff).Post("/lessons", h.Lesson.Create)
				r.Get("/lessons", h.Lesson.ListByClass)
			})

			// Lessons and words
			r.Get("/lessons", h.Lesson.ListByDate)
			r.Route("/lessons/{id}", func(r chi.Router) {
				r.Get("/", h.Lesson.Get)
				r.With(staff).Patch("/", h.Lesson.Update)
				r.With(staff).Delete("/", h.Lesson.Delete)
				r.With(staff).Post("/words", h.Lesson.AddWord)
				r.With(staff).Post("/words/reorder", h.Lesson.ReorderWords)
			})
			r.With(staff).Patch("/words/{id}", h.Lesson.UpdateWord)
			r.With(staff).Delete("/words/{id}", h.Lesson.DeleteWord)
			r.With(staff).Put("/words/{id}/audio", h.Lesson.UploadAudio)
			r.With(staff).Delete("/words/{id}/audio", h.Lesson.DeleteAudio)

			// Pronunciation (students record; teachers/admins can hear an attempt)
			r.Route("/pronunciation", func(r chi.Router) {
				r.Use(auth.RequireRole(user.RoleStudent))
				r.Get("/history", h.Pronunciation.History)
				r.Post("/{wordId}", h.Pronunciation.Score)
				r.Get("/{id}/audio", h.Pronunciation.Audio)
			})

			// Teacher dashboard (own classes; admin sees all)
			r.Route("/teacher", func(r chi.Router) {
				r.Use(staff)
				r.Get("/overview", h.Teacher.Overview)
				r.Get("/classes/{id}/progress", h.Teacher.Progress)
				r.Get("/classes/{id}/students/{studentId}", h.Teacher.Student)
			})

			// Admin
			r.Route("/admin", func(r chi.Router) {
				r.Use(adminOnly)
				r.Get("/overview", h.Admin.Overview)
				r.Get("/users", h.Admin.ListUsers)
				r.Post("/users", h.Admin.CreateUser)
				r.Patch("/users/{id}/role", h.Admin.UpdateRole)
				r.Post("/users/{id}/disable", h.Admin.Disable)
				r.Post("/users/{id}/enable", h.Admin.Enable)
			})
		})
	})

	return r
}

func healthHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		status, code := "ok", http.StatusOK
		if err := db.Ping(ctx); err != nil {
			status, code = "database unavailable", http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(map[string]string{"status": status})
	}
}
