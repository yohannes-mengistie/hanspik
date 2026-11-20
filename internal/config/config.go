package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
	UploadDir   string
	Scorer      string
	AzureKey    string
	AzureRegion string

	// Set both to create (or repair) an admin account at startup. This is the
	// only way to get the first admin: the admin API needs an admin token.
	BootstrapAdminEmail    string
	BootstrapAdminPassword string
	BootstrapAdminName     string
}

func Load() Config {
	// Loads .env if it exists. In production you set real env vars instead.
	_ = godotenv.Load()

	return Config{
		Port:        getEnv("PORT", "8084"),
		DatabaseURL: mustEnv("DATABASE_URL"),
		JWTSecret:   mustEnv("JWT_SECRET"),
		UploadDir:   getEnv("UPLOAD_DIR", "uploads"),
		Scorer:      getEnv("SCORER", "fake"),
		AzureKey:    getEnv("AZURE_SPEECH_KEY", ""),
		AzureRegion: getEnv("AZURE_SPEECH_REGION", ""),

		BootstrapAdminEmail:    strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_EMAIL")),
		BootstrapAdminPassword: os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"),
		BootstrapAdminName:     getEnv("BOOTSTRAP_ADMIN_NAME", "Admin"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required environment variable: %s", key)
	}
	return v
}
