package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/service"
)

type contextKey string

const UserKey contextKey = "auth_user"

// RequireAuth is an HTTP middleware that validates the Bearer token.
// On success it stores the authenticated User in the request context.
// On failure it replies 401 and aborts the chain.
func RequireAuth(svc *service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := bearerToken(r)
			u, err := svc.VerifyToken(r.Context(), token)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"code":"unauthorized","message":"invalid or expired token"}`))
				return
			}
			ctx := context.WithValue(r.Context(), UserKey, u)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserFromContext retrieves the authenticated user placed by RequireAuth.
func UserFromContext(ctx context.Context) (model.User, bool) {
	u, ok := ctx.Value(UserKey).(model.User)
	return u, ok
}

// BearerToken extracts the raw token from Authorization header.
func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
}

// RequestID injects / forwards a request-id header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = time.Now().UTC().Format("20060102150405.000000000")
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r)
	})
}
