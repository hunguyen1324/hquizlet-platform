package middleware

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

var internalSigningKey = os.Getenv("INTERNAL_SIGNING_KEY")

func verifyInternalSignature(userID, role, tsStr, sig string) bool {
	if internalSigningKey == "" || sig == "" {
		return false
	}
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix()-ts > 30 || time.Now().Unix()-ts < -30 {
		return false
	}
	msg := userID + "|" + role + "|" + tsStr
	mac := hmac.New(sha256.New, []byte(internalSigningKey))
	mac.Write([]byte(msg))
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(sig), []byte(expectedSig))
}

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

			rawUserID := r.Header.Get("X-User-ID")
			rawRole := r.Header.Get("X-User-Role")
			rawTs := r.Header.Get("X-Internal-Ts")
			rawSig := r.Header.Get("X-Internal-Sig")

			r.Header.Del("X-User-ID")
			r.Header.Del("X-User-Role")

			if verifyInternalSignature(rawUserID, rawRole, rawTs, rawSig) {
				r.Header.Set("X-User-ID", rawUserID)
				r.Header.Set("X-User-Role", rawRole)
				w.Header().Set("Cache-Control", "private, no-store")
				next.ServeHTTP(w, r)
				return
			}

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
