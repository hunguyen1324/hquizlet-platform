package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAudioTicketIntegrityAndExpiry(t *testing.T) {
	want := audioTicket{SetID: 3, UserID: 7, Session: "session", Index: 2, Expires: time.Now().Add(time.Minute).Unix()}
	raw := signAudioTicket(want)
	got, ok := parseAudioTicket(raw)
	if !ok || got != want {
		t.Fatalf("ticket round trip: %+v %v", got, ok)
	}
	for _, bad := range []string{"", raw + "tampered", "e30.invalid", signAudioTicket(audioTicket{SetID: 3, UserID: 7, Index: 2, Expires: time.Now().Add(-time.Second).Unix()})} {
		if _, ok := parseAudioTicket(bad); ok {
			t.Fatal("accepted forged/expired ticket")
		}
		recorder := httptest.NewRecorder()
		(&Handler{}).streamQuizAudio(recorder, httptest.NewRequest(http.MethodGet, "/v1/quiz-audio?ticket="+bad, nil))
		if recorder.Code != 401 {
			t.Fatalf("invalid ticket status %d", recorder.Code)
		}
	}
}
