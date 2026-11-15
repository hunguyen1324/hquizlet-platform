package youtubecaptions

import (
	"context"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestCaptionURLCannotFetchOtherHostsOrVideos(t *testing.T) {
	for _, raw := range []string{"http://www.youtube.com/api/timedtext?v=M7lc1UVf-VE", "https://www.youtube.com.evil.test/api/timedtext?v=M7lc1UVf-VE", "https://127.0.0.1/api/timedtext?v=M7lc1UVf-VE", "https://www.youtube.com:444/api/timedtext?v=M7lc1UVf-VE", "https://user@www.youtube.com/api/timedtext?v=M7lc1UVf-VE", "https://www.youtube.com/redirect?v=M7lc1UVf-VE", "https://www.youtube.com/api/timedtext?v=dQw4w9WgXcQ"} {
		if _, err := safeCaptionURL(raw, "M7lc1UVf-VE", ""); err == nil {
			t.Fatal("accepted", raw)
		}
	}
}
func TestPublicCaptionLoadTranslationAndCache(t *testing.T) {
	client := New()
	calls := 0
	client.HTTP.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		body := ""
		switch r.URL.Path {
		case "/watch":
			body = `<script>var ytInitialPlayerResponse = {"captions":{"playerCaptionsTracklistRenderer":{"captionTracks":[{"baseUrl":"https://www.youtube.com/api/timedtext?v=M7lc1UVf-VE&lang=ja&signature=example","languageCode":"ja","isTranslatable":true,"name":{"simpleText":"日本語"}}]}}};</script>`
		case "/api/timedtext":
			if r.URL.Query().Get("fmt") != "json3" || r.URL.Query().Get("signature") != "example" {
				t.Fatal(r.URL)
			}
			if r.URL.Query().Get("tlang") == "vi" {
				body = `{"events":[{"tStartMs":0,"dDurationMs":2000,"segs":[{"utf8":"Xin chào"}]}]}`
			} else {
				body = `{"events":[{"tStartMs":0,"dDurationMs":2000,"segs":[{"utf8":"こんにちは"}]}]}`
			}
		default:
			t.Fatal(r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})
	for i := 0; i < 2; i++ {
		result, err := client.Fetch(context.Background(), "M7lc1UVf-VE", "ja", "vi")
		if err != nil {
			t.Fatal(err)
		}
		if result.Original.Cues[0].Text != "こんにちは" || result.Translation.Cues[0].Text != "Xin chào" {
			t.Fatal(result)
		}
	}
	if calls != 3 {
		t.Fatal("cache missed", calls)
	}
}
func TestCaptionChallengeEmptyOrInvalidDoesNotInventTranscript(t *testing.T) {
	for _, body := range []string{"", `<html>sign in to continue</html>`, `var ytInitialPlayerResponse = {"captions":{}};`} {
		if _, err := extractTracks([]byte(body)); err == nil {
			t.Fatal(body)
		}
	}
	if _, err := decodeJSON3([]byte(`{"events":[{"tStartMs":1,"dDurationMs":0,"segs":[{"utf8":"empty timing"}]}]}`)); err == nil {
		t.Fatal("invalid timing accepted")
	}
}
func TestCueLimitsAndCleanup(t *testing.T) {
	cues := []Cue{{2, 3, "<b>Hello &amp; world</b>"}, {0, 1, "こんにちは"}}
	if err := ValidateCues(cues); err != nil {
		t.Fatal(err)
	}
	if cues[0].Start != 0 || cues[1].Text != "Hello & world" {
		t.Fatal(cues)
	}
	for _, cue := range []Cue{{-1, 2, "bad"}, {0, 0, "bad"}, {0, 1, ""}, {0, 43201, "bad"}, {math.NaN(), 1, "bad"}, {0, math.Inf(1), "bad"}, {0, 1, "\x00"}} {
		if ValidateCues([]Cue{cue}) == nil {
			t.Fatal("invalid cue accepted", cue)
		}
	}
}
