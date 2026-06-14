package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	DatabaseURL string
	UploadDir   string
}

func Load() Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found! using system environment variables")
	}

	port := getEnv("PORT", "8080")
	databaseURL := getEnv("DATABASE_URL", "postgres://user:password@localhost:5432/mydb?sslmode=disable")
	uploadDir := getEnv("UPLOAD_DIR", "./uploads")

	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required but not set")
	}

	return Config{
		Port:        port,
		DatabaseURL: databaseURL,
		UploadDir:   uploadDir,
	}
}

func getEnv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
