package http

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// internalTokenOK compares X-Internal-Token against INTERNAL_TOKEN.
// INTERNAL_TOKEN must match the caller's token (CLASS_INTERNAL_TOKEN for the class service);
// the default keeps local development working without extra configuration.
func internalTokenOK(r *http.Request) bool {
	want := os.Getenv("INTERNAL_TOKEN")
	if want == "" {
		want = "dev-internal-token"
	}
	got := r.Header.Get("X-Internal-Token")
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// parseIDList parses "1,2,3" into int64 ids, skipping blanks and non-positive values.
func parseIDList(raw string) []int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]int64, 0, len(parts))
	for _, p := range parts {
		id, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
		if err != nil || id <= 0 {
			continue
		}
		out = append(out, id)
	}
	return out
}
