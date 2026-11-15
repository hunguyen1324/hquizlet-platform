package youtubecaptions

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeExtractor(t *testing.T, partial bool) (*YTDLP, *int, *string) {
	t.Helper()
	calls := 0
	dirSeen := ""
	tool := &YTDLP{Binary: "fake"}
	tool.run = func(ctx context.Context, dir string, args []string) ([]byte, error) {
		calls++
		dirSeen = dir
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, "--ignore-config") || !strings.Contains(joined, "--skip-download") {
			t.Fatal("unsafe command", args)
		}
		if calls == 1 {
			return []byte(`{"id":"J2cmxVjMZtc","language":"ja","subtitles":{"ja":[{}]},"automatic_captions":{"ja":[{}],"ja-orig":[{}],"vi":[{}]}}`), nil
		}
		if !strings.Contains(joined, "^ja$,^vi$") || !strings.Contains(joined, "--load-info-json") {
			t.Fatal(args)
		}
		if err := os.WriteFile(filepath.Join(dir, "caption.ja.json3"), []byte(`{"events":[{"tStartMs":0,"dDurationMs":2000,"segs":[{"utf8":"こんにちは"}]}]}`), 0600); err != nil {
			t.Fatal(err)
		}
		if partial {
			return nil, errors.New("translation unavailable")
		}
		if err := os.WriteFile(filepath.Join(dir, "caption.vi.json3"), []byte(`{"events":[{"tStartMs":0,"dDurationMs":2000,"segs":[{"utf8":"Xin chào"}]}]}`), 0600); err != nil {
			t.Fatal(err)
		}
		return nil, nil
	}
	return tool, &calls, &dirSeen
}
func TestYTDLPSubtitlesAndCleanup(t *testing.T) {
	for _, partial := range []bool{false, true} {
		tool, calls, dir := fakeExtractor(t, partial)
		result, err := tool.Fetch(context.Background(), "J2cmxVjMZtc", "", "vi")
		if err != nil || result.Original == nil || result.Original.Cues[0].Text != "こんにちは" || *calls != 2 {
			t.Fatal(result, err, *calls)
		}
		if (result.Translation == nil) != partial {
			t.Fatal("translation handling", result)
		}
		if _, err := os.Stat(*dir); !os.IsNotExist(err) {
			t.Fatal("temporary files leaked")
		}
		if result.AvailableTracks[0].Automatic {
			t.Fatal("manual track must take precedence")
		}
	}
}
func TestYTDLPRejectsInvalidInputAndMetadata(t *testing.T) {
	tool := &YTDLP{Binary: "fake", run: func(context.Context, string, []string) ([]byte, error) {
		t.Fatal("must not execute invalid input")
		return nil, nil
	}}
	for _, id := range []string{"--exec=bad", "https://evil.test/", "../a"} {
		if _, err := tool.Fetch(context.Background(), id, "ja", "vi"); err == nil {
			t.Fatal(id)
		}
	}
	tool.run = func(context.Context, string, []string) ([]byte, error) {
		return []byte(`{"id":"otherVideo1","subtitles":{"ja":[{}]}}`), nil
	}
	if _, err := tool.Fetch(context.Background(), "J2cmxVjMZtc", "ja", ""); err == nil {
		t.Fatal("wrong video accepted")
	}
	buffer := &limitedBuffer{limit: 3}
	if _, err := buffer.Write([]byte("1234")); err == nil {
		t.Fatal("unbounded stdout")
	}
}
func TestClientFallsBackAndCachesResult(t *testing.T) {
	c := New()
	tool, calls, _ := fakeExtractor(t, false)
	c.Extractor = tool
	c.HTTP.Transport = roundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("no player response")), Header: http.Header{}}, nil
	})
	for i := 0; i < 2; i++ {
		r, err := c.Fetch(context.Background(), "J2cmxVjMZtc", "ja", "vi")
		if err != nil || r.Original == nil {
			t.Fatal(r, err)
		}
	}
	if *calls != 2 {
		t.Fatal("fallback cache missed", *calls)
	}
}
