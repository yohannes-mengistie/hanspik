package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/yohannes/chines-backend/internal/auth"
	"github.com/yohannes/chines-backend/internal/class"
	"github.com/yohannes/chines-backend/internal/config"
	"github.com/yohannes/chines-backend/internal/lesson"
	"github.com/yohannes/chines-backend/internal/platform/database"
	"github.com/yohannes/chines-backend/internal/platform/storage"
	"github.com/yohannes/chines-backend/internal/practice"
	"github.com/yohannes/chines-backend/internal/server"
	"github.com/yohannes/chines-backend/internal/user"
)

func main() {
	cfg := config.Load()

	db, err := database.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()
	userRepo := user.NewRepository(db)
	authRepo := auth.NewRepository(db)
	classRepo := class.NewRepository(db)
	lessonRepo := lesson.NewRepository(db)
	tokens := auth.NewTokenManager(cfg.JWTSecret, 15*time.Minute)
	authSvc := auth.NewService(userRepo, authRepo, tokens, 30*24*time.Hour)
	classSvc := class.NewService(classRepo, userRepo)
	practiceRepo := practice.NewRepository(db)
	practiceSvc := practice.NewService(practiceRepo, lessonRepo, classRepo)

	audioStore, err := storage.NewLocal(filepath.Join(cfg.UploadDir, "audio"))
	if err != nil {
		log.Fatalf("storage: %v", err)
	}
	lessonSvc := lesson.NewService(lessonRepo, classRepo, audioStore)

	handlers := server.Handlers{
		Auth:     auth.NewHandler(authSvc),
		Class:    class.NewHandler(classSvc),
		Lesson:   lesson.NewHandler(lessonSvc),
		Practice: practice.NewHandler(practiceSvc),
		Media:    audioStore.Serve,
	}
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           server.NewRouter(db, tokens, handlers),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Start the server in a goroutine so main can wait for a stop signal.
	go func() {
		log.Printf("server listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	// Wait for Ctrl+C or a termination signal.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	log.Println("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}