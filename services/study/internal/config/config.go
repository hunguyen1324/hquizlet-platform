package config

import "os"

// Config holds all study service configuration read from environment.
type Config struct {
	Port        string
	DatabaseURL string
	AuthSecret  string // shared secret for verifying auth tokens from auth service
}

// Load reads configuration from environment variables with sane defaults.
func Load() Config {
	return Config{
		Port:        env("PORT", "8082"),
		DatabaseURL: env("DATABASE_URL", "postgres://hquizlet:hquizlet@localhost:5432/hquizlet?sslmode=disable"),
		AuthSecret:  env("AUTH_SECRET", ""),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
