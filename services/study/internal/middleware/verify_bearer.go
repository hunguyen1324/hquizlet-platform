package middleware

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// VerifyBearer authenticates even direct /v1 calls; X-User-ID is not proof of identity.
// Internal service endpoints must remain on the private service network.
func VerifyBearer(authURL string) func(http.Handler) http.Handler {
	client := &http.Client{Timeout: 3 * time.Second}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.URL.Path, "/v1/") || strings.HasPrefix(r.URL.Path, "/v1/templates/") || ((r.Method == "GET" || r.Method == "HEAD") && r.URL.Path == "/v1/quiz-audio") {
				next.ServeHTTP(w, r)
				return
			}
			r.Header.Del("X-User-ID")
			r.Header.Del("X-User-Role")
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") || strings.TrimSpace(strings.TrimPrefix(auth, "Bearer ")) == "" {
				http.Error(w, "authentication required", 401)
				return
			}
			req, err := http.NewRequestWithContext(r.Context(), "GET", strings.TrimRight(authURL, "/")+"/internal/auth/verify", nil)
			if err != nil {
				http.Error(w, "authentication unavailable", 503)
				return
			}
			req.Header.Set("Authorization", auth)
			resp, err := client.Do(req)
			if err != nil {
				http.Error(w, "authentication unavailable", 503)
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode == 401 {
				http.Error(w, "authentication required", 401)
				return
			}
			if resp.StatusCode != 200 {
				http.Error(w, "authentication unavailable", 503)
				return
			}
			var identity struct {
				Authenticated bool   `json:"authenticated"`
				UserID        int64  `json:"userId"`
				Role          string `json:"role"`
			}
			if json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&identity) != nil || !identity.Authenticated || identity.UserID <= 0 {
				http.Error(w, "authentication required", 401)
				return
			}
			r.Header.Set("X-User-ID", strconv.FormatInt(identity.UserID, 10))
			r.Header.Set("X-User-Role", identity.Role)
			w.Header().Set("Cache-Control", "private, no-store")
			next.ServeHTTP(w, r)
		})
	}
}
