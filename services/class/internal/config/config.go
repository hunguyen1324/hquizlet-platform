// Package config holds class service configuration read from environment.
package config

import "os"

// Config holds all class service configuration.
type Config struct {
	Port              string
	DatabaseURL       string
	AuthServiceURL    string
	StudyServiceURL   string
	NATSUrl           string
	ClassInternalToken string
}

// Load reads configuration from environment variables with sane defaults.
func Load() Config {
	return Config{
		Port:              env("PORT", "8084"),
		DatabaseURL:       env("DATABASE_URL", "postgres://hquizlet:hquizlet@localhost:5432/hquizlet?sslmode=disable"),
		AuthServiceURL:    env("AUTH_SERVICE_URL", "http://localhost:8081"),
		StudyServiceURL:   env("STUDY_SERVICE_URL", "http://localhost:8082"),
		NATSUrl:           env("NATS_URL", "nats://localhost:4222"),
		ClassInternalToken: env("CLASS_INTERNAL_TOKEN", "dev-internal-token"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
