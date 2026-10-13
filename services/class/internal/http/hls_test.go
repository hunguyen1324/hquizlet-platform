package http

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHLSConversionEncryptsSegmentsAndExcludesKeys(t *testing.T) {
	checkHLSEncryption(t, realVideoFixture(t))
}

func checkHLSEncryption(t *testing.T, input []byte) {
	t.Helper()
	h, _, store := privateTestHandler(t, "owner")
	h.videoMaxBytes = 1024 * 1024
	object := strings.Repeat("a", 32) + ".mp4"
	if err := h.putHLS(context.Background(), object, bytes.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(store.body), int64(len(store.body)))
	if err != nil {
		t.Fatal(err)
	}
	var playlist string
	var encrypted []byte
	for _, entry := range zr.File {
		if entry.Name != "index.m3u8" && !hlsSegmentName.MatchString(entry.Name) {
			t.Fatalf("sensitive file in bundle: %s", entry.Name)
		}
		f, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if entry.Name == "index.m3u8" {
			playlist = string(data)
		} else {
			encrypted = data
		}
	}
	if !strings.Contains(playlist, "METHOD=AES-128") || !strings.Contains(playlist, `URI="key"`) || len(encrypted) == 0 {
		t.Fatal("missing encrypted HLS assets")
	}
	start := strings.Index(playlist, "IV=0x")
	if start < 0 {
		t.Fatal("missing IV")
	}
	iv, err := hex.DecodeString(playlist[start+5 : start+37])
	if err != nil {
		t.Fatal(err)
	}
	block, _ := aes.NewCipher(hlsKey(h.hlsSecret, object))
	plain := make([]byte, len(encrypted))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, encrypted)
	if plain[0] != 0x47 || plain[188] != 0x47 {
		t.Fatal("AES output is not MPEG-TS")
	}
	if bytes.Equal(hlsKey(h.hlsSecret, object), hlsKey(h.hlsSecret, strings.Repeat("b", 32)+".mp4")) {
		t.Fatal("videos share key")
	}
}

func TestHLSManifestRejectsExternalPathsAndRewritesSignedAssets(t *testing.T) {
	valid := "#EXTM3U\n#EXT-X-KEY:METHOD=AES-128,URI=\"key\",IV=0x00000000000000000000000000000000\nsegment000000.ts\n#EXT-X-ENDLIST\n"
	out, err := rewriteHLS(valid, "signed-ticket")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, "ticket=signed-ticket") != 2 || !strings.Contains(out, "asset=key") {
		t.Fatal(out)
	}
	for _, bad := range []string{strings.Replace(valid, "segment000000.ts", "../key", 1), strings.Replace(valid, "segment000000.ts", "https://evil.test/video.ts", 1), strings.Replace(valid, "AES-128", "NONE", 1), "#EXTM3U\nsegment000000.ts"} {
		if _, err := rewriteHLS(bad, "x"); err == nil {
			t.Fatal("accepted unsafe manifest")
		}
	}
}

func TestHLSRejectsSharedExpiredAndTamperedTickets(t *testing.T) {
	h, mock, store := privateTestHandler(t, "student")
	signed := signVideoTicket(h.videoSecret, videoTicket{2, 1, 3, time.Now().Add(time.Minute).Unix()})
	for _, tc := range []struct{ uid, raw string }{{"1", signed}, {"2", signed + "x"}, {"2", signVideoTicket(h.videoSecret, videoTicket{2, 1, 3, time.Now().Add(-time.Minute).Unix()})}, {"", signed}} {
		r := httptest.NewRequest("GET", "/v1/class-video-hls?ticket="+tc.raw+"&asset=key", nil)
		r.Header.Set("X-User-ID", tc.uid)
		w := httptest.NewRecorder()
		h.serveHLS(w, r)
		if w.Code != 401 {
			t.Fatalf("got %d", w.Code)
		}
	}
	if store.opened {
		t.Fatal("unauthorized storage read")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHLSKeyRechecksVideoPermission(t *testing.T) {
	h, mock, store := privateTestHandler(t, "student")
	for _, allowed := range []bool{false, true} {
		q := mock.ExpectQuery("SELECT storage_key,content_type").WithArgs(int64(1), int64(3), false, "[2]")
		rows := sqlmock.NewRows([]string{"storage_key", "content_type"})
		if allowed {
			rows.AddRow(strings.Repeat("a", 32)+".mp4", hlsContentType)
		}
		q.WillReturnRows(rows)
		signed := signVideoTicket(h.videoSecret, videoTicket{2, 1, 3, time.Now().Add(time.Minute).Unix()})
		r := httptest.NewRequest("GET", "/v1/class-video-hls?ticket="+signed+"&asset=key", nil)
		r.Header.Set("X-User-ID", "2")
		w := httptest.NewRecorder()
		h.serveHLS(w, r)
		if allowed {
			if w.Code != 200 || w.Body.Len() != 16 {
				t.Fatal("missing key")
			}
		} else if w.Code != 404 {
			t.Fatalf("revoked viewer got %d", w.Code)
		}
		if w.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal("key cacheable")
		}
	}
	if store.opened {
		t.Fatal("key must not be read from SFTP")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
