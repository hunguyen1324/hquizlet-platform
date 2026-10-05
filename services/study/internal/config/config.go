package config

import "os"

// Config holds all study service configuration read from environment.
type Config struct {
	AuthServiceURL    string
	PaymentServiceURL string
	AudioAllowedHosts string
	Port              string
	DatabaseURL       string
	RedisURL          string
	AuthSecret        string // shared secret for verifying auth tokens from auth service
	MinIO             MinIOConfig
}

// MinIOConfig holds S3-compatible storage for async import file buffering.
type MinIOConfig struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	// LocalSpoolDir is used when Endpoint is empty (dev without MinIO).
	LocalSpoolDir string
}

// Load reads configuration from environment variables with sane defaults.
func Load() Config {
	return Config{
		AuthServiceURL:    env("AUTH_SERVICE_URL", "http://localhost:8081"),
		PaymentServiceURL: env("PAYMENT_SERVICE_URL", "http://localhost:8085"),
		AudioAllowedHosts: env("QUIZ_AUDIO_ALLOWED_HOSTS", ""),
		Port:              env("PORT", "8082"),
		DatabaseURL:       env("DATABASE_URL", "postgres://hquizlet:hquizlet@localhost:5432/hquizlet?sslmode=disable"),
		RedisURL:          env("REDIS_URL", "redis://localhost:6379"),
		AuthSecret:        env("AUTH_SECRET", ""),
		MinIO: MinIOConfig{
			Endpoint:        env("MINIO_ENDPOINT", ""),
			AccessKeyID:     env("MINIO_ACCESS_KEY", env("STORAGE_ACCESS_KEY", "minioadmin")),
			SecretAccessKey: env("MINIO_SECRET_KEY", env("STORAGE_SECRET_KEY", "minioadmin")),
			Bucket:          env("MINIO_IMPORT_BUCKET", "hquizlet-imports"),
			LocalSpoolDir:   env("IMPORT_SPOOL_DIR", os.TempDir()+"/hquizlet-imports"),
		},
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
