package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSTTClientReadsCompletedSubtitle(t *testing.T) {
	id := strings.Repeat("a", 32)
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer server-secret" {
			t.Error("STT secret missing")
			w.WriteHeader(401)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/srt") {
			w.Write([]byte("1\n00:00:00,000 --> 00:00:01,000\nXin chào\n"))
			return
		}
		WriteJSON(w, 200, map[string]string{"id": id, "status": "completed"})
	}))
	defer remote.Close()
	h := (&Handler{}).WithSTT(remote.URL, "server-secret")
	view, err := h.transcription(context.Background(), id)
	if err != nil || view.Status != "completed" || !strings.Contains(view.SRTText, "Xin chào") {
		t.Fatalf("%+v %v", view, err)
	}
}
func TestSTTClientRejectsMismatchedJobAndInvalidSRT(t *testing.T) {
	for _, test := range []struct{ name, id, srt string }{{"other job", strings.Repeat("b", 32), "1\n00:00:00,000 --> 00:00:01,000\ntext\n"}, {"invalid subtitle", strings.Repeat("a", 32), "not subtitle"}} {
		t.Run(test.name, func(t *testing.T) {
			remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/srt") {
					w.Write([]byte(test.srt))
					return
				}
				WriteJSON(w, 200, map[string]string{"id": test.id, "status": "completed"})
			}))
			defer remote.Close()
			h := (&Handler{}).WithSTT(remote.URL, "secret")
			if _, err := h.transcription(context.Background(), strings.Repeat("a", 32)); err == nil {
				t.Fatal("invalid response accepted")
			}
		})
	}
}
