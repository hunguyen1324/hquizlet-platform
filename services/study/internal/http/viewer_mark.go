package http

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base32"
	"net/http"
	"strings"
)

// WithViewerMarkSecret enables GET /v1/study-sets/viewer-mark. The mark is a
// pseudonymous, server-derived code (HMAC of the authenticated user id) used
// for screenshot traceability. It is not a credential and is NOT accepted for
// authentication. Operators can map a leaked code back to an account with
// ViewerMark(secret, userID) over candidate ids.
func (h *Handler) WithViewerMarkSecret(secret string) *Handler {
	h.markSecret = []byte(secret)
	return h
}

// ViewerMark derives the short watermark code for a user id.
func ViewerMark(secret []byte, uid int64) string {
	m := hmac.New(sha256.New, secret)
	var b [8]byte
	for i := 0; i < 8; i++ {
		b[i] = byte(uid >> (8 * i))
	}
	m.Write(b[:])
	return strings.ToUpper(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(m.Sum(nil)[:5]))
}

func (h *Handler) viewerMark(w http.ResponseWriter, r *http.Request) {
	protectedHeaders(w)
	uid := userIDFromHeader(r)
	if uid < 1 {
		WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if len(h.markSecret) == 0 {
		WriteError(w, http.StatusServiceUnavailable, "watermark disabled")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"mark": ViewerMark(h.markSecret, uid)})
}
