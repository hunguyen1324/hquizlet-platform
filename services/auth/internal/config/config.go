package config

import (
	"os"
	"time"
)

type Config struct {
	Port        string
	DatabaseURL string
	SessionTTL  time.Duration
}

func Load() Config {
	ttl := 30 * 24 * time.Hour
	if raw := os.Getenv("SESSION_TTL"); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil {
			ttl = d
		}
	}
	return Config{
		Port:        env("PORT", "8081"),
		DatabaseURL: env("DATABASE_URL", "postgres://hquizlet:hquizlet@localhost:5432/hquizlet?sslmode=disable"),
		SessionTTL:  ttl,
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
