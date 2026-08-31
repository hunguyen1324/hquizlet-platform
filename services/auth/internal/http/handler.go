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

// NewRouter wires up all auth routes.
func NewRouter(svc *service.AuthService, db *sql.DB) http.Handler {
	mux := http.NewServeMux()

	// health
	mux.HandleFunc("GET /healthz", healthHandler("auth", db))

	// auth lifecycle – P2-AUTH-01
	mux.HandleFunc("GET /v1/auth/me", meHandler(svc))
	mux.HandleFunc("POST /v1/auth/register", registerHandler(svc))
	mux.HandleFunc("POST /v1/auth/login", loginHandler(svc))
	mux.HandleFunc("POST /v1/auth/logout", logoutHandler(svc))
	mux.HandleFunc("POST /v1/auth/logout-all", logoutAllHandler(svc))
	mux.HandleFunc("POST /v1/auth/refresh", refreshHandler(svc))

	// internal token verify for gateway/study – P2-AUTH-02
	mux.HandleFunc("GET /internal/auth/verify", verifyHandler(svc))

	// user profile – P2-AUTH-03
	mux.HandleFunc("GET /v1/auth/profile", profileGetHandler(svc))
	mux.HandleFunc("PATCH /v1/auth/profile", profilePatchHandler(svc))

	return loggingMiddleware(requestIDMiddleware(mux))
}

// --- handlers ---

func meHandler(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, err := svc.Me(r.Context(), bearerToken(r))
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
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid JSON body")
			return
		}
		resp, err := svc.Register(r.Context(), input)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, resp)
	}
}

func loginHandler(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input model.LoginInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid JSON body")
			return
		}
		resp, err := svc.Login(r.Context(), input)
		if err != nil {
			writeServiceError(w, err)
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

// logoutAllHandler – P2-AUTH-01: invalidates all sessions for this user.
func logoutAllHandler(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := svc.LogoutAll(r.Context(), bearerToken(r)); err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

func refreshHandler(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := svc.Refresh(r.Context(), bearerToken(r))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// verifyHandler – P2-AUTH-02: internal endpoint for gateway/study to verify tokens.
// Returns user identity so study service can read userId from X-Auth-User-Id header
// set by gateway after calling this endpoint, OR reads directly via this endpoint.
func verifyHandler(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, err := svc.VerifyToken(r.Context(), bearerToken(r))
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"authenticated": true,
			"userId":        u.ID,
			"email":         u.Email,
			"name":          u.Name,
			"role":          u.Role,
		})
	}
}

// profileGetHandler – P2-AUTH-03: get current user's profile.
func profileGetHandler(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, err := svc.GetProfile(r.Context(), bearerToken(r))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, u)
	}
}

// profilePatchHandler – P2-AUTH-03: update name/image.
func profilePatchHandler(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input model.UpdateProfileInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid JSON body")
			return
		}
		u, err := svc.UpdateProfile(r.Context(), bearerToken(r), input)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, u)
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

// --- P2-AUTH-04: standardized error responses ---
//
// Canonical error envelope shared across ALL Auth endpoints.
// Dev2 (Study) and Dev5 (Integration) must adopt the same shape.
//
// Normal error:
//   { "code": "<snake_case_code>", "message": "<human readable>" }
//
// Validation error (422 only) — superset of the normal envelope:
//   { "code": "validation_error", "message": "<reason>", "field": "<field_name>" }

// apiError is the canonical error envelope for all Auth error responses.
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// apiValidationError extends apiError with a field name for 422 responses.
// It is a strict superset of apiError so clients that only read code/message still work.
type apiValidationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field"`
}

// writeError sends a structured JSON error using the canonical envelope.
func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, apiError{Code: code, Message: msg})
}

// writeServiceError maps service-layer errors to correct HTTP status + canonical code.
// P2-AUTH-04: every branch must emit {code, message} — never a bare string or {error}.
func writeServiceError(w http.ResponseWriter, err error) {
	var ve *service.ValidationError
	switch {
	case errors.As(err, &ve):
		writeJSON(w, http.StatusUnprocessableEntity, apiValidationError{
			Code:    "validation_error",
			Message: ve.Msg,
			Field:   ve.Field,
		})
	case errors.Is(err, service.ErrEmailTaken):
		writeError(w, http.StatusConflict, "email_taken", "email already registered")
	case errors.Is(err, service.ErrInvalidCredential):
		writeError(w, http.StatusUnauthorized, "invalid_credential", "invalid email or password")
	case errors.Is(err, service.ErrInvalidSession):
		writeError(w, http.StatusUnauthorized, "invalid_session", "invalid or expired session")
	case errors.Is(err, service.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "access denied")
	default:
		log.Printf("[auth] unexpected error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("[auth] writeJSON encode error: %v", err)
	}
}

// --- middleware ---

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
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
