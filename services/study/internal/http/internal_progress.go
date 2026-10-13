package http

import (
	"crypto/subtle"
	"github.com/hunguyen1324/hquizlet-platform/packages/authz"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// internalProgressSummary serves GET /internal/progress/summary for the class dashboard.
// The endpoint stays on the private service network and additionally requires the shared
// internal token so a mis-routed proxy cannot read other users' progress.
func (h *Handler) internalProgressSummary(w http.ResponseWriter, r *http.Request) {
	if !authz.VerifyServiceRequest(r, os.Getenv("INTERNAL_SIGNING_KEY")) {
		WriteError(w, http.StatusUnauthorized, "invalid service signature")
		return
	}
	if h.stats == nil {
		WriteError(w, http.StatusServiceUnavailable, "statistics unavailable")
		return
	}

	setIDs := parseIDList(r.URL.Query().Get("studySetIds"))
	userIDs := parseIDList(r.URL.Query().Get("userIds"))
	if len(setIDs) == 0 || len(userIDs) == 0 {
		WriteError(w, http.StatusBadRequest, "studySetIds and userIds are required")
		return
	}

	items, err := h.stats.ProgressSummary(r.Context(), setIDs, userIDs)
	if err != nil {
		WriteServiceError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

// internalTokenOK compares X-Internal-Token against INTERNAL_TOKEN.
// INTERNAL_TOKEN must match the caller's CLASS_INTERNAL_TOKEN; the default keeps local
// development working without extra configuration.
func internalTokenOK(r *http.Request) bool {
	want := os.Getenv("INTERNAL_TOKEN")
	if want == "" {
		want = "dev-internal-token"
	}
	got := r.Header.Get("X-Internal-Token")
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// parseIDList parses "1,2,3" into int64 ids, skipping blanks and negatives.
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
