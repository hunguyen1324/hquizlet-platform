package middleware

import (
	"bytes"
	"encoding/json"
	"github.com/hunguyen1324/hquizlet-platform/packages/authz"
	"net/http"
	"os"
	"strings"
	"time"
)

// RequireReauthentication checks the actor's current session/password with auth before a money write.
func RequireReauthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		password := r.Header.Get("X-Admin-Password")
		if password == "" {
			writeError(w, 401, "REAUTH_REQUIRED", "password confirmation required")
			return
		}
		target := os.Getenv("AUTH_SERVICE_URL")
		if target == "" {
			target = "http://localhost:8081"
		}
		body, _ := json.Marshal(map[string]string{"password": password})
		req, err := http.NewRequestWithContext(r.Context(), "POST", strings.TrimRight(target, "/")+"/v1/auth/reauth", bytes.NewReader(body))
		if err != nil {
			writeError(w, 503, "AUTH_UNAVAILABLE", "confirmation unavailable")
			return
		}
		req.Header.Set("Authorization", r.Header.Get("Authorization"))
		req.Header.Set("Content-Type", "application/json")
		resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
		if err != nil {
			writeError(w, 503, "AUTH_UNAVAILABLE", "confirmation unavailable")
			return
		}
		defer resp.Body.Close()
		var verified struct {
			UserID        int64    `json:"userId"`
			Role          string   `json:"role"`
			PlatformRoles []string `json:"platformRoles"`
		}
		if resp.StatusCode != 200 || json.NewDecoder(resp.Body).Decode(&verified) != nil || verified.UserID != GetUserID(r.Context()) {
			writeError(w, 401, "REAUTH_REQUIRED", "password confirmation failed")
			return
		}
		if !authz.Can(authz.Principal{UserID: verified.UserID, LegacyRole: verified.Role, PlatformRoles: verified.PlatformRoles}, authz.ActionWalletCredit, authz.Resource{Type: "wallet"}) {
			writeError(w, 403, "FORBIDDEN", "wallet credit permission required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
