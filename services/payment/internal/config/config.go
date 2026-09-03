package config

import (
	"os"
	"strings"
)

type Config struct {
	Port               string
	DatabaseURL        string
	StudyServiceURL    string
	AuthServiceURL     string
	SePayAPIToken      string
	SePayBankAccountID string
	SePayVAAccount     string
	SePayWebhookAPIKey string
	SePayBaseURL       string
	SePayAllowedIPs    string
	AdminToken         string
}

func Load() Config {
	return Config{
		Port:               env("PORT", "8085"),
		DatabaseURL:        env("DATABASE_URL", "postgres://hquizlet:hquizlet@localhost:5432/hquizlet?sslmode=disable"),
		StudyServiceURL:    env("STUDY_SERVICE_URL", "http://localhost:8082"),
		AuthServiceURL:     env("AUTH_SERVICE_URL", "http://localhost:8081"),
		SePayAPIToken:      cleanEnv(os.Getenv("SEPAY_API_TOKEN")),
		SePayBankAccountID: cleanEnv(os.Getenv("SEPAY_BIDV_BANK_ACCOUNT_ID")),
		SePayVAAccount:     cleanEnv(os.Getenv("SEPAY_VA_ACCOUNT_NUMBER")),
		SePayWebhookAPIKey: cleanEnv(os.Getenv("SEPAY_WEBHOOK_API_KEY")),
		SePayBaseURL:       env("SEPAY_API_BASE_URL", "https://userapi.sepay.vn/v2"),
		SePayAllowedIPs:    os.Getenv("SEPAY_ALLOWED_IPS"),
		AdminToken:         env("ADMIN_TOKEN", "dev-admin-token"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		if cleaned := cleanEnv(v); cleaned != "" {
			return cleaned
		}
	}
	return fallback
}

func cleanEnv(v string) string {
	return strings.Trim(strings.TrimSpace(v), `"'`)
}
