package http

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/service"
)

// NewRouter wires up all auth routes and returns a ready *http.ServeMux.
func NewRouter(svc *service.AuthService, db *sql.DB) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", healthHandler("auth", db))
	mux.HandleFunc("GET /v1/auth/me", meHandler(svc))
	mux.HandleFunc("POST /v1/auth/register", registerHandler(svc))
	mux.HandleFunc("POST /v1/auth/login", loginHandler(svc))
	mux.HandleFunc("POST /v1/auth/logout", logoutHandler(svc))
	mux.HandleFunc("POST /v1/auth/refresh", refreshHandler(svc))

	return loggingMiddleware(requestIDMiddleware(mux))
}

// --- handlers ---

func meHandler(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		u, err := svc.Me(r.Context(), token)
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]any{"authenticated": false, "user": nil})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "user": u})
	}
}

func registerHandler(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input model.RegisterInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		resp, err := svc.Register(r.Context(), input)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, service.ErrEmailTaken) {
				status = http.StatusConflict
			}
			writeError(w, status, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, resp)
	}
}

func loginHandler(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input model.LoginInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		resp, err := svc.Login(r.Context(), input)
		if err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func logoutHandler(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		svc.Logout(r.Context(), bearerToken(r))
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

func refreshHandler(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := svc.Refresh(r.Context(), bearerToken(r))
		if err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
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

// --- response helpers (BE-AUTH-06) ---

// ErrorResponse is the canonical error envelope.
type ErrorResponse struct {
	Error string `json:"error"`
}

// SuccessEnvelope wraps successful data (optional, used where needed).
type SuccessEnvelope struct {
	Data any `json:"data"`
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, ErrorResponse{Error: msg})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("writeJSON encode error: %v", err)
	}
}

// --- inline middleware (logging / request-id) ---

func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = time.Now().UTC().Format("20060102150405.000000000")
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("[auth] %s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}
