package http

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// A short-lived capability for one delivered audio, never an account bearer token.
// The key lives only for this process; a restart safely invalidates old links.
var audioTicketKey = func() []byte {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}()

type audioTicket struct {
	SetID   int64  `json:"s"`
	UserID  int64  `json:"u"`
	Session string `json:"q"`
	Index   int    `json:"i"`
	Expires int64  `json:"e"`
}

func signAudioTicket(t audioTicket) string {
	data, _ := json.Marshal(t)
	payload := base64.RawURLEncoding.EncodeToString(data)
	mac := hmac.New(sha256.New, audioTicketKey)
	mac.Write([]byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func parseAudioTicket(raw string) (audioTicket, bool) {
	var ticket audioTicket
	parts := strings.Split(raw, ".")
	if len(parts) != 2 || len(raw) > 1024 {
		return ticket, false
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ticket, false
	}
	mac := hmac.New(sha256.New, audioTicketKey)
	mac.Write([]byte(parts[0]))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return ticket, false
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ticket, false
	}
	if json.Unmarshal(data, &ticket) != nil {
		return ticket, false
	}
	return ticket, ticket.Expires > time.Now().Unix() && ticket.UserID > 0 && ticket.SetID > 0 && ticket.Index >= 0
}

func (h *Handler) streamQuizAudio(w http.ResponseWriter, r *http.Request) {
	protectedHeaders(w)
	ticket, ok := parseAudioTicket(r.URL.Query().Get("ticket"))
	if !ok {
		WriteError(w, 401, "Liên kết nghe đã hết hạn. Hãy tải lại âm thanh.")
		return
	}
	if h.protectedQuiz == nil || h.quizAudio == nil {
		WriteError(w, 503, "audio unavailable")
		return
	}
	// Recheck current entitlement and session on every Range request.
	source, err := h.protectedQuiz.Audio(r.Context(), ticket.SetID, ticket.UserID, ticket.Session, ticket.Index)
	if err != nil {
		h.playError(w, r, err)
		return
	}
	reader, mime, err := h.quizAudio.Open(r.Context(), ticket.SetID, source)
	if err != nil {
		WriteError(w, 422, "Không tải được âm thanh.")
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Disposition", "inline")
	http.ServeContent(w, r, "audio", time.Time{}, reader)
}
