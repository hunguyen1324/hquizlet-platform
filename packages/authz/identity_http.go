package authz

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// VerifyIdentity accepts a signed gateway identity or validates the bearer with auth.
// It replaces all client-supplied principal headers, including organization membership.
func VerifyIdentity(authURL string, protect func(*http.Request) bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !protect(r) {
				next.ServeHTTP(w, r)
				return
			}
			uid, role, roles, orgs := r.Header.Get("X-User-ID"), r.Header.Get("X-User-Role"), r.Header.Get("X-User-Roles"), r.Header.Get("X-User-Org-Roles")
			stamp, sig := r.Header.Get("X-Internal-Ts"), r.Header.Get("X-Internal-Sig")
			valid := false
			key := os.Getenv("INTERNAL_SIGNING_KEY")
			ts, err := strconv.ParseInt(stamp, 10, 64)
			id, iderr := strconv.ParseInt(uid, 10, 64)
			if key != "" && err == nil && iderr == nil && id > 0 && time.Now().Unix()-ts <= 30 && time.Now().Unix()-ts >= -30 {
				messages := []string{uid + "|" + role + "|" + roles + "|" + orgs + "|" + stamp}
				if orgs == "" {
					messages = append(messages, uid+"|"+role+"|"+roles+"|"+stamp)
				}
				for _, msg := range messages {
					mac := hmac.New(sha256.New, []byte(key))
					mac.Write([]byte(msg))
					if hmac.Equal([]byte(sig), []byte(hex.EncodeToString(mac.Sum(nil)))) {
						valid = true
					}
				}
			}
			for _, header := range []string{"X-User-ID", "X-User-Role", "X-User-Roles", "X-User-Org-Roles", "X-Class-Role", "X-Member-ID"} {
				r.Header.Del(header)
			}
			if !valid {
				if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
					http.Error(w, "authentication required", 401)
					return
				}
				req, err := http.NewRequestWithContext(r.Context(), "GET", strings.TrimRight(authURL, "/")+"/internal/auth/verify", nil)
				if err != nil {
					http.Error(w, "authentication unavailable", 503)
					return
				}
				req.Header.Set("Authorization", r.Header.Get("Authorization"))
				resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
				if err != nil {
					http.Error(w, "authentication unavailable", 503)
					return
				}
				defer resp.Body.Close()
				var identity struct {
					Authenticated bool     `json:"authenticated"`
					UserID        int64    `json:"userId"`
					Role          string   `json:"role"`
					PlatformRoles []string `json:"platformRoles"`
					OrgRoles      []string `json:"orgRoles"`
				}
				if resp.StatusCode != 200 || json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&identity) != nil || !identity.Authenticated || identity.UserID < 1 {
					http.Error(w, "authentication required", 401)
					return
				}
				uid = strconv.FormatInt(identity.UserID, 10)
				role = identity.Role
				roles = strings.Join(identity.PlatformRoles, ",")
				orgs = strings.Join(identity.OrgRoles, ",")
			}
			r.Header.Set("X-User-ID", uid)
			r.Header.Set("X-User-Role", role)
			r.Header.Set("X-User-Roles", roles)
			r.Header.Set("X-User-Org-Roles", orgs)
			w.Header().Set("Cache-Control", "private, no-store")
			next.ServeHTTP(w, r)
		})
	}
}
