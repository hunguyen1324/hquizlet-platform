package storage

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func hexID(seed byte) string {
	return strings.Repeat(strconv.FormatUint(uint64(seed), 16), 64)[:64]
}

func TestDiskCacheRoundTrip(t *testing.T) {
	cache, err := newDiskCache(t.TempDir(), DefaultAudioCacheBytes)
	if err != nil {
		t.Fatal(err)
	}
	id := hexID(0xab)
	if _, err := cache.open(id); err == nil {
		t.Fatal("expected a cache miss")
	}
	data := payload(4096)
	stored, err := cache.store(id, data)
	if err != nil {
		t.Fatal(err)
	}
	defer stored.Close()
	got, err := io.ReadAll(stored)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("stored copy = %d bytes, %v", len(got), err)
	}
	// No half-written file may survive a store.
	entries, err := os.ReadDir(cache.dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != id {
		t.Fatalf("cache dir = %v, %v", entries, err)
	}
	rc, err := cache.open(id)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	defer rc.Close()
	if got, err = io.ReadAll(rc); err != nil || !bytes.Equal(got, data) {
		t.Fatalf("cached copy = %d bytes, %v", len(got), err)
	}
}

func TestDiskCacheRejectsUnsafeIDs(t *testing.T) {
	cache, err := newDiskCache(t.TempDir(), DefaultAudioCacheBytes)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"", "abc", "../../etc/passwd", strings.Repeat("z", 64), strings.Repeat("A", 64)} {
		if _, err := cache.store(id, []byte("x")); err == nil {
			t.Fatalf("stored unsafe id %q", id)
		}
		if _, err := cache.open(id); err == nil {
			t.Fatalf("opened unsafe id %q", id)
		}
	}
	entries, err := os.ReadDir(cache.dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("unsafe ids created files: %v, %v", entries, err)
	}
}

func TestDiskCacheEvictsOldestBeyondByteCap(t *testing.T) {
	cache, err := newDiskCache(t.TempDir(), 150)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Now().Add(-time.Hour)
	ids := []string{hexID(1), hexID(2), hexID(3)}
	for i, id := range ids {
		stored, err := cache.store(id, payload(60))
		if err != nil {
			t.Fatal(err)
		}
		stored.Close()
		if err := os.Chtimes(filepath.Join(cache.dir, id), base.Add(time.Duration(i)*time.Minute), base.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	var total int64
	entries, err := os.ReadDir(cache.dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			t.Fatal(err)
		}
		total += info.Size()
	}
	if total > 150 {
		t.Fatalf("cache holds %d bytes, cap is 150", total)
	}
	if _, err := os.Stat(filepath.Join(cache.dir, ids[0])); !os.IsNotExist(err) {
		t.Fatal("oldest entry was not evicted")
	}
	if _, err := os.Stat(filepath.Join(cache.dir, ids[2])); err != nil {
		t.Fatalf("newest entry was evicted: %v", err)
	}
}

func TestDiskCacheKeepsOversizedSoleEntryAndSweepsPartFiles(t *testing.T) {
	cache, err := newDiskCache(t.TempDir(), 100)
	if err != nil {
		t.Fatal(err)
	}
	id := hexID(7)
	stored, err := cache.store(id, payload(400))
	if err != nil {
		t.Fatal(err)
	}
	stored.Close()
	if rc, err := cache.open(id); err != nil {
		t.Fatalf("oversized sole entry dropped: %v", err)
	} else {
		rc.Close()
	}
	// A crash can leave a part file behind; it must never be served.
	stale := filepath.Join(cache.dir, hexID(8)+partSuffix)
	if err := os.WriteFile(stale, []byte("half"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(stale, time.Now().Add(-time.Hour), time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	filler, err := cache.store(hexID(9), payload(16))
	if err != nil {
		t.Fatal(err)
	}
	filler.Close()
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("stale part file was not swept")
	}
	if rc, err := cache.open(hexID(8)); err == nil {
		rc.Close()
		t.Fatal("a part file was served")
	}
	// An empty file is a corrupted entry, not content.
	empty := filepath.Join(cache.dir, hexID(10))
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.open(hexID(10)); err == nil {
		t.Fatal("empty cache entry was served")
	}
	if _, err := os.Stat(empty); !os.IsNotExist(err) {
		t.Fatal("empty cache entry was kept")
	}
}

// A disk copy is served through the same Range path as any other reader.
func TestDiskCacheThroughServeContent(t *testing.T) {
	cache, err := newDiskCache(t.TempDir(), DefaultAudioCacheBytes)
	if err != nil {
		t.Fatal(err)
	}
	data := payload(1024)
	id := hexID(0xcd)
	stored, err := cache.store(id, data)
	if err != nil {
		t.Fatal(err)
	}
	stored.Close()
	reader, err := cache.open(id)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	req := httptest.NewRequest(http.MethodGet, "/v1/quiz-audio", nil)
	req.Header.Set("Range", "bytes=10-19")
	rec := httptest.NewRecorder()
	http.ServeContent(rec, req, "audio", time.Time{}, reader)
	if rec.Code != http.StatusPartialContent || !bytes.Equal(rec.Body.Bytes(), data[10:20]) {
		t.Fatalf("cached range: status %d, %q", rec.Code, rec.Body.Bytes())
	}
	if rec.Header().Get("Content-Range") != "bytes 10-19/1024" {
		t.Fatalf("content range = %q", rec.Header().Get("Content-Range"))
	}
}
