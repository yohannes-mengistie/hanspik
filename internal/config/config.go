package config

import (
	"log"
	"os"

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
