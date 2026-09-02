package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type contextKey string

const (
	UserIDKey   contextKey = "payment_user_id"
	UserRoleKey contextKey = "payment_user_role"
)

// Auth reads X-User-ID and X-User-Role from gateway-injected headers.
func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userIDStr := r.Header.Get("X-User-ID")
		if userIDStr == "" {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
			return
		}
		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil || userID <= 0 {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid user id")
			return
		}
		role := r.Header.Get("X-User-Role")
		if role == "" {
			role = "user"
		}
		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		ctx = context.WithValue(ctx, UserRoleKey, role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAdmin checks that the user has admin role.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role, _ := r.Context().Value(UserRoleKey).(string)
		if role != "admin" {
			writeError(w, http.StatusForbidden, "FORBIDDEN", "admin access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// OptionalAuth reads user ID if present but doesn't require it.
func OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userIDStr := r.Header.Get("X-User-ID")
		if userIDStr != "" {
			if userID, err := strconv.ParseInt(userIDStr, 10, 64); err == nil && userID > 0 {
				ctx := context.WithValue(r.Context(), UserIDKey, userID)
				role := r.Header.Get("X-User-Role")
				if role == "" {
					role = "user"
				}
				ctx = context.WithValue(ctx, UserRoleKey, role)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// GetUserID extracts the user ID from context.
func GetUserID(ctx context.Context) int64 {
	id, _ := ctx.Value(UserIDKey).(int64)
	return id
}

// GetUserRole extracts the user role from context.
func GetUserRole(ctx context.Context) string {
	role, _ := ctx.Value(UserRoleKey).(string)
	return role
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code":    code,
		"message": message,
		"details": map[string]any{},
	})
}

// AdminTokenAuth is an alternative admin auth for direct admin operations.
func AdminTokenAuth(adminToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check X-Admin-Token header
			adminHeader := r.Header.Get("X-Admin-Token")
			if adminHeader != "" && adminHeader == adminToken {
				ctx := context.WithValue(r.Context(), UserRoleKey, "admin")
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			// Fall through to normal auth
			role, _ := r.Context().Value(UserRoleKey).(string)
			if role == "admin" {
				next.ServeHTTP(w, r)
				return
			}
			writeError(w, http.StatusForbidden, "FORBIDDEN", "admin access required")
		})
	}
}

// StripSpoofedHeaders removes any client-supplied identity headers.
func StripSpoofedHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Del("X-User-ID")
		r.Header.Del("X-User-Role")
		r.Header.Del("X-Participant-ID")
		r.Header.Del("X-Live-Role")
		r.Header.Del("X-Class-Role")
		r.Header.Del("X-Member-ID")
		next.ServeHTTP(w, r)
	})
}

// BearerToken extracts the raw token from Authorization header.
func BearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
}
