package config

import (
	"os"

	"github.com/joho/godotenv"
)

// Config holds process settings. Empty DatabaseURL means the API runs without Postgres.
type Config struct {
	Addr        string
	DatabaseURL string
}

// Load reads .env from the working directory, then the process environment.
// A variable already set in the shell is left unchanged.
func Load() Config {
	_ = godotenv.Load()
	return Config{
		Addr:        getenv("ADDR", ":8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
