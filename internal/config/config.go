package config

import "os"

// Config holds process settings. Empty DatabaseURL means the API runs without Postgres.
type Config struct {
	Addr        string
	DatabaseURL string
}

// Load reads configuration from the environment.
func Load() Config {
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
