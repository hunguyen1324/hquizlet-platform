// Package middleware provides HTTP middleware for the study service.
package middleware

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"
)

// RequestID injects or propagates an X-Request-ID header.
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

// Logging logs method, path, and elapsed time for every request.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("[study] %s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}

// RequireUserID protects study resources from unauthenticated requests.
// The gateway/auth layer is responsible for authenticating the bearer token and
// injecting X-User-ID. Health endpoints remain public because this middleware
// only applies to /v1/ routes.
func RequireUserID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Path) < 4 || r.URL.Path[:4] != "/v1/" {
			next.ServeHTTP(w, r)
			return
		}

		raw := r.Header.Get("X-User-ID")
		userID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || userID <= 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "authentication required"})
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Chain applies a list of middleware in order (first = outermost).
func Chain(h http.Handler, mw ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h
}
