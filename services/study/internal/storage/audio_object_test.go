package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// memStore is an in-memory ObjectRangeStore that records how many bytes each
// range request actually transferred, and can truncate a window the way a
// server closing the connection early does.
type memStore struct {
	data  []byte
	limit int // maximum bytes delivered per request, 0 = unlimited
	fail  error

	mu       sync.Mutex
	requests [][2]int64
	served   int
}

func (m *memStore) Stat(_ context.Context, key string) (int64, error) {
	if m.fail != nil {
		return 0, m.fail
	}
	if key != "quiz-audio/x" {
		return 0, ErrObjectNotFound
	}
	return int64(len(m.data)), nil
}

func (m *memStore) Get(_ context.Context, key string, start, length int64) (io.ReadCloser, error) {
	if m.fail != nil {
		return nil, m.fail
	}
	if key != "quiz-audio/x" || start < 0 || start >= int64(len(m.data)) {
		return nil, ErrObjectNotFound
	}
	end := int64(len(m.data))
	if length >= 0 && start+length < end {
		end = start + length
	}
	m.mu.Lock()
	m.requests = append(m.requests, [2]int64{start, end})
	m.mu.Unlock()
	window := m.data[start:end]
	if m.limit > 0 && len(window) > m.limit {
		window = window[:m.limit]
	}
	return &countingReader{Reader: bytes.NewReader(window), store: m}, nil
}

type countingReader struct {
	io.Reader
	store *memStore
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.Reader.Read(p)
	c.store.mu.Lock()
	c.store.served += n
	c.store.mu.Unlock()
	return n, err
}

func (c *countingReader) Close() error { return nil }

func payload(size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i*31 + 7)
	}
	return data
}

func newTestReader(t *testing.T, store ObjectRangeStore) io.ReadSeekCloser {
	t.Helper()
	r, err := newRangeReader(context.Background(), store, "quiz-audio/x")
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// The bytes=0-0 warm-up must confirm existence without pulling the file.
func TestAudioRangeWarmUpTransfersOneByte(t *testing.T) {
	store := &memStore{data: payload(1 << 20)}
	reader := newTestReader(t, store)
	defer reader.Close()

	size, err := reader.Seek(0, io.SeekEnd)
	if err != nil || size != int64(len(store.data)) {
		t.Fatalf("size = %d, %v", size, err)
	}
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 1)
	n, err := reader.Read(buf)
	if err != nil || n != 1 {
		t.Fatalf("warm-up read = %d, %v", n, err)
	}
	if store.served != 1 {
		t.Fatalf("warm-up transferred %d bytes, want 1", store.served)
	}
	if len(store.requests) != 1 || store.requests[0] != [2]int64{0, 1} {
		t.Fatalf("range requests = %v", store.requests)
	}
}

func TestAudioRangeReaderSeekAndEOF(t *testing.T) {
	data := payload(1000)
	store := &memStore{data: data}
	reader := newTestReader(t, store)
	defer reader.Close()

	if pos, err := reader.Seek(-1, io.SeekStart); err == nil || pos != 0 {
		t.Fatalf("negative seek accepted: %d %v", pos, err)
	}
	if _, err := reader.Seek(0, 99); err == nil {
		t.Fatal("invalid whence accepted")
	}
	if pos, err := reader.Seek(int64(len(data)-4), io.SeekStart); err != nil || pos != int64(len(data)-4) {
		t.Fatalf("seek start: %d %v", pos, err)
	}
	tail := make([]byte, 16)
	n, err := reader.Read(tail)
	if err != nil || n != 4 || !bytes.Equal(tail[:n], data[len(data)-4:]) {
		t.Fatalf("tail read = %d, %v, %q", n, err, tail[:n])
	}
	if n, err := reader.Read(tail); n != 0 || !errors.Is(err, io.EOF) {
		t.Fatalf("read past end = %d, %v", n, err)
	}
	if pos, err := reader.Seek(0, io.SeekEnd); err != nil || pos != int64(len(data)) {
		t.Fatalf("seek end: %d %v", pos, err)
	}
	if pos, err := reader.Seek(10, io.SeekEnd); err != nil || pos != int64(len(data)+10) {
		t.Fatalf("seek past end: %d %v", pos, err)
	}
	if n, err := reader.Read(tail); n != 0 || !errors.Is(err, io.EOF) {
		t.Fatalf("read past end after seek = %d, %v", n, err)
	}
	// A seek abandons the open window so the next read refetches from pos.
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	if n, err := reader.Read(tail[:5]); err != nil || n != 5 || !bytes.Equal(tail[:5], data[:5]) {
		t.Fatalf("restart read = %d, %v", n, err)
	}
}

func TestAudioRangeReaderSurvivesTruncatedWindow(t *testing.T) {
	data := payload(2000)
	// The store answers every request with at most 3 bytes: reads must keep
	// reopening windows instead of treating the early close as the end.
	store := &memStore{data: data, limit: 3}
	reader := newTestReader(t, store)
	defer reader.Close()

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("truncated stream length %d, want %d", len(got), len(data))
	}
	if len(store.requests) < len(data)/3 {
		t.Fatalf("expected a window per chunk, got %d requests", len(store.requests))
	}
}

func TestAudioRangeReaderErrors(t *testing.T) {
	if _, err := newRangeReader(context.Background(), &memStore{fail: errors.New("minio down")}, "quiz-audio/x"); err == nil {
		t.Fatal("stat failure ignored")
	}
	if _, err := newRangeReader(context.Background(), &memStore{data: payload(8)}, "other"); !errors.Is(err, ErrObjectNotFound) {
		t.Fatalf("missing object error = %v", err)
	}
	store := &memStore{data: payload(8)}
	reader := newTestReader(t, store)
	reader.Close()
	if _, err := reader.Read(make([]byte, 1)); err == nil {
		t.Fatal("read after close accepted")
	}
}

// http.ServeContent is what the handlers call: ranges must survive the trip.
func TestAudioRangeThroughServeContent(t *testing.T) {
	data := payload(4096)
	for _, tc := range []struct {
		rangeHeader string // Range header value, empty for a full GET
		status      int
		want        []byte
	}{
		{"", http.StatusOK, data},
		{"bytes=0-0", http.StatusPartialContent, data[:1]},
		{"bytes=100-199", http.StatusPartialContent, data[100:200]},
		{"bytes=4090-", http.StatusPartialContent, data[4090:]},
	} {
		store := &memStore{data: data}
		reader := newTestReader(t, store)
		req := httptest.NewRequest(http.MethodGet, "/v1/quiz-audio", nil)
		if tc.rangeHeader != "" {
			req.Header.Set("Range", tc.rangeHeader)
		}
		rec := httptest.NewRecorder()
		// The handlers set the type themselves, so ServeContent never has to
		// sniff (a sniff would read 512 bytes through the range reader).
		rec.Header().Set("Content-Type", "audio/mpeg")
		http.ServeContent(rec, req, "audio", time.Time{}, reader)
		reader.Close()
		if rec.Code != tc.status || !bytes.Equal(rec.Body.Bytes(), tc.want) {
			t.Fatalf("range %q: status %d, %d bytes, want %d", tc.rangeHeader, rec.Code, rec.Body.Len(), len(tc.want))
		}
		if rec.Header().Get("Accept-Ranges") != "bytes" {
			t.Fatalf("range %q missing Accept-Ranges", tc.rangeHeader)
		}
		if tc.rangeHeader != "" && rec.Header().Get("Content-Range") == "" {
			t.Fatalf("range %q missing Content-Range", tc.rangeHeader)
		}
		// Only the bytes the client asked for left the store.
		if store.served != len(tc.want) {
			t.Fatalf("range %q transferred %d bytes, want %d", tc.rangeHeader, store.served, len(tc.want))
		}
	}
}

// Sequential playback grows the window, so a whole file costs a handful of
// requests rather than one request per buffer.
func TestAudioRangeReaderRampsWindow(t *testing.T) {
	data := payload(1 << 20)
	store := &memStore{data: data}
	reader := newTestReader(t, store)
	defer reader.Close()

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("full read length %d, want %d", len(got), len(data))
	}
	if len(store.requests) > 16 {
		t.Fatalf("full 1 MiB stream used %d range requests, want a handful", len(store.requests))
	}
	if store.served != len(data) {
		t.Fatalf("stream transferred %d bytes, want %d", store.served, len(data))
	}
}
