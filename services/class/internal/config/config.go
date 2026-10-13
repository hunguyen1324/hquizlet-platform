// Package config holds class service configuration read from environment.
package config

import (
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/videostorage"
	"os"
	"strconv"
)

// Config holds all class service configuration.
type Config struct {
	Port               string
	DatabaseURL        string
	AuthServiceURL     string
	StudyServiceURL    string
	NATSUrl            string
	ClassInternalToken string
	VideoStorage       videostorage.Config
	VideoTicketSecret  string
	VideoMaxBytes      int64
}

// Load reads configuration from environment variables with sane defaults.
func Load() Config {
	return Config{
		Port:               env("PORT", "8084"),
		DatabaseURL:        env("DATABASE_URL", "postgres://hquizlet:hquizlet@localhost:5432/hquizlet?sslmode=disable"),
		AuthServiceURL:     env("AUTH_SERVICE_URL", "http://localhost:8081"),
		StudyServiceURL:    env("STUDY_SERVICE_URL", "http://localhost:8082"),
		NATSUrl:            env("NATS_URL", "nats://localhost:4222"),
		ClassInternalToken: env("CLASS_INTERNAL_TOKEN", "dev-internal-token"),
		VideoStorage: videostorage.Config{
			Host: os.Getenv("VIDEO_SFTP_HOST"), Port: envNumber("VIDEO_SFTP_PORT", 2022),
			User: os.Getenv("VIDEO_SFTP_USER"), Root: env("VIDEO_SFTP_ROOT", "/private-videos"),
			HostFingerprint: os.Getenv("VIDEO_SFTP_HOST_FINGERPRINT"),
			Password:        os.Getenv("VIDEO_SFTP_PASSWORD"), PasswordFile: os.Getenv("VIDEO_SFTP_PASSWORD_FILE"),
			PrivateKeyFile: os.Getenv("VIDEO_SFTP_PRIVATE_KEY_FILE"), MaxConnections: envNumber("VIDEO_SFTP_MAX_CONNECTIONS", 4),
		},
		VideoTicketSecret: os.Getenv("VIDEO_TICKET_SECRET"),
		VideoMaxBytes:     int64(envNumber("VIDEO_MAX_UPLOAD_MB", 80)) * 1024 * 1024,
	}
}

// Invalid explicit values fail validation rather than silently falling back.
func envNumber(key string, fallback int) int {
	if os.Getenv(key) == "" {
		return fallback
	}
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return -1
	}
	return value
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
