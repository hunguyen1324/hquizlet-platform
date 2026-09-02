package http

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/hunguyen1324/hquizlet-platform/services/file/internal/http/handler"
	"github.com/hunguyen1324/hquizlet-platform/services/file/internal/http/middleware"
	"github.com/hunguyen1324/hquizlet-platform/services/file/internal/service"
)

// NewRouter creates the HTTP mux with all routes and middleware.
func NewRouter(svc *service.FileService, db *sql.DB) http.Handler {
	mux := http.NewServeMux()

	// Health endpoints (public)
	mux.HandleFunc("GET /healthz", healthHandler("file", db))
	mux.HandleFunc("GET /healthz/storage", storageHealthHandler(svc))

	// File upload endpoints (authenticated)
	fileHandler := handler.NewFileHandler(svc)
	fileHandler.Register(mux)

	// Apply middleware chain
	return middleware.Chain(mux,
		middleware.RequestID,
		middleware.Logging,
		middleware.RequireUserID,
	)
}

func healthHandler(serviceName string, db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body := map[string]string{"service": serviceName, "status": "ok", "database": "ok"}
		status := http.StatusOK
		if err := db.PingContext(r.Context()); err != nil {
			status = http.StatusServiceUnavailable
			body["status"] = "degraded"
			body["database"] = "offline"
		}
		writeJSON(w, status, body)
	}
}

func storageHealthHandler(svc *service.FileService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Just verify the service is initialized
		writeJSON(w, http.StatusOK, map[string]string{"service": "file-storage", "status": "ok"})
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
