package authz

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestIdentityProofCannotBeReusedForDifferentRoles(t *testing.T) {
	t.Setenv("INTERNAL_SIGNING_KEY", "test-key")
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"authenticated":true,"userId":9,"role":"user","platformRoles":["Student"]}`))
	}))
	defer auth.Close()
	for _, tamper := range []string{"platform", "organization", "expired", "none"} {
		t.Run(tamper, func(t *testing.T) {
			ts := strconv.FormatInt(time.Now().Unix(), 10)
			if tamper == "expired" {
				ts = strconv.FormatInt(time.Now().Add(-time.Hour).Unix(), 10)
			}
			msg := "9|user|Teacher|" + ts
			mac := hmac.New(sha256.New, []byte("test-key"))
			mac.Write([]byte(msg))
			sig := hex.EncodeToString(mac.Sum(nil))
			r := httptest.NewRequest("GET", "/v1/test", nil)
			r.Header.Set("X-User-ID", "9")
			r.Header.Set("X-User-Role", "user")
			r.Header.Set("X-User-Roles", "Teacher")
			r.Header.Set("X-Internal-Ts", ts)
			r.Header.Set("X-Internal-Sig", sig)
			r.Header.Set("Authorization", "Bearer token")
			if tamper == "platform" {
				r.Header.Set("X-User-Roles", "PlatformAdmin")
			}
			if tamper == "organization" {
				r.Header.Set("X-User-Org-Roles", "7:OrgAdmin")
			}
			expected := "Student"
			if tamper == "none" {
				expected = "Teacher"
			}
			h := VerifyIdentity(auth.URL, func(*http.Request) bool { return true })(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("X-User-Roles") != expected || r.Header.Get("X-User-Org-Roles") != "" {
					t.Fatalf("unverified identity: %v", r.Header)
				}
				w.WriteHeader(204)
			}))
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != 204 {
				t.Fatalf("status %d", w.Code)
			}
		})
	}
}

func TestIdentityRejectsUnsignedHeadersWithoutBearer(t *testing.T) {
	h := VerifyIdentity("http://unused", func(*http.Request) bool { return true })(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("unauthorized request reached handler") }))
	r := httptest.NewRequest("GET", "/v1/test", nil)
	r.Header.Set("X-User-ID", "1")
	r.Header.Set("X-User-Roles", "PlatformAdmin")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatalf("status %d", w.Code)
	}
}
