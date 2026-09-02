package middleware

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// RequireUserID reads X-User-ID injected by the gateway.
// Health endpoints remain public because this middleware only applies to /v1/ routes.
func RequireUserID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Health endpoints are public
		if len(r.URL.Path) < 4 || r.URL.Path[:4] != "/v1/" {
			next.ServeHTTP(w, r)
			return
		}
		raw := r.Header.Get("X-User-ID")
		userID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || userID <= 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"code": "UNAUTHORIZED", "message": "authentication required"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// GetUserID extracts the user ID from the X-User-ID header.
func GetUserID(r *http.Request) int64 {
	raw := r.Header.Get("X-User-ID")
	id, _ := strconv.ParseInt(raw, 10, 64)
	return id
}
