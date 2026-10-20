package storage

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeS3 answers HEAD/GET the way MinIO does, honouring Range requests, and
// records the Range header of every GET.
type fakeS3 struct {
	bucket  string
	key     string
	object  []byte
	mu      sync.Mutex
	ranges  []string
	served  int
	missReq int
}

func (f *fakeS3) handler(w http.ResponseWriter, r *http.Request) {
	path := "/" + f.bucket + "/" + f.key
	if r.URL.Path != path {
		f.mu.Lock()
		f.missReq++
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `<?xml version="1.0"?><Error><Code>NoSuchKey</Code><Message>missing</Message></Error>`)
		return
	}
	if r.Method == http.MethodHead {
		w.Header().Set("Content-Length", strconv.Itoa(len(f.object)))
		w.WriteHeader(http.StatusOK)
		return
	}
	start, end := int64(0), int64(len(f.object))-1
	ranged := false
	if spec := r.Header.Get("Range"); spec != "" {
		f.mu.Lock()
		f.ranges = append(f.ranges, spec)
		f.mu.Unlock()
		var ok bool
		start, end, ok = parseByteRange(spec, int64(len(f.object)))
		if !ok {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		ranged = true
	}
	body := f.object[start : end+1]
	f.mu.Lock()
	f.served += len(body)
	f.mu.Unlock()
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	if ranged {
		w.Header().Set("Content-Range", "bytes "+strconv.FormatInt(start, 10)+"-"+strconv.FormatInt(end, 10)+"/"+strconv.Itoa(len(f.object)))
		w.WriteHeader(http.StatusPartialContent)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	w.Write(body)
}

func parseByteRange(spec string, size int64) (int64, int64, bool) {
	if !strings.HasPrefix(spec, "bytes=") {
		return 0, 0, false
	}
	parts := strings.Split(strings.TrimPrefix(spec, "bytes="), "-")
	if len(parts) != 2 || parts[0] == "" {
		return 0, 0, false
	}
	start, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || start >= size {
		return 0, 0, false
	}
	end := size - 1
	if parts[1] != "" {
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil || end < start {
			return 0, 0, false
		}
		if end >= size {
			end = size - 1
		}
	}
	return start, end, true
}

func newFakeS3(t *testing.T, key string, object []byte) (*fakeS3, *httptest.Server) {
	t.Helper()
	fake := &fakeS3{bucket: "hquizlet", key: key, object: object}
	server := httptest.NewServer(http.HandlerFunc(fake.handler))
	t.Cleanup(server.Close)
	return fake, server
}

func s3Store(t *testing.T, server *httptest.Server) ObjectRangeStore {
	t.Helper()
	store, err := NewMinIOImportStorage(MinIOConfig{
		Endpoint:        server.URL,
		AccessKeyID:     "minioadmin",
		SecretAccessKey: "minioadmin",
		Bucket:          "hquizlet",
	})
	if err != nil {
		t.Fatal(err)
	}
	objects, ok := store.(ObjectRangeStore)
	if !ok {
		t.Fatal("minio storage does not implement ObjectRangeStore")
	}
	return objects
}

func TestMinIOStoreReadsByteRanges(t *testing.T) {
	object := payload(1000)
	fake, server := newFakeS3(t, "quiz-audio/abc", object)
	objects := s3Store(t, server)
	ctx := context.Background()

	size, err := objects.Stat(ctx, "quiz-audio/abc")
	if err != nil || size != int64(len(object)) {
		t.Fatalf("stat = %d, %v", size, err)
	}
	rc, err := objects.Get(ctx, "quiz-audio/abc", 10, 5)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(rc)
	rc.Close()
	if err != nil || !bytes.Equal(got, object[10:15]) {
		t.Fatalf("ranged read = %q, %v", got, err)
	}
	if len(fake.ranges) != 1 || fake.ranges[0] != "bytes=10-14" {
		t.Fatalf("range headers = %v", fake.ranges)
	}
	if fake.served != 5 {
		t.Fatalf("server transferred %d bytes, want 5", fake.served)
	}

	// Open-ended range: used when a caller streams to the end.
	rc, err = objects.Get(ctx, "quiz-audio/abc", 990, -1)
	if err != nil {
		t.Fatal(err)
	}
	got, err = io.ReadAll(rc)
	rc.Close()
	if err != nil || !bytes.Equal(got, object[990:]) {
		t.Fatalf("open-ended read = %q, %v", got, err)
	}
	if fake.ranges[len(fake.ranges)-1] != "bytes=990-" {
		t.Fatalf("range headers = %v", fake.ranges)
	}

	if _, err := objects.Stat(ctx, "quiz-audio/missing"); err == nil {
		t.Fatal("missing object reported as present")
	}
	if _, err := objects.Get(ctx, "quiz-audio/missing", 0, 10); err == nil {
		t.Fatal("missing object read succeeded")
	}
}

// The whole handler path — Range request, seekable reader, ServeContent —
// against a real S3 client, without ever holding the object in memory.
func TestMinIOStoreThroughServeContent(t *testing.T) {
	object := payload(64 << 10)
	fake, server := newFakeS3(t, "quiz-audio/abc", object)
	objects := s3Store(t, server)

	reader, err := newRangeReader(context.Background(), objects, "quiz-audio/abc")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	req := httptest.NewRequest(http.MethodGet, "/v1/quiz-audio", nil)
	req.Header.Set("Range", "bytes=0-0")
	rec := httptest.NewRecorder()
	rec.Header().Set("Content-Type", "audio/mpeg")
	http.ServeContent(rec, req, "audio", time.Time{}, reader)
	if rec.Code != http.StatusPartialContent || !bytes.Equal(rec.Body.Bytes(), object[:1]) {
		t.Fatalf("warm-up: status %d, %d bytes", rec.Code, rec.Body.Len())
	}
	if len(fake.ranges) != 1 || fake.ranges[0] != "bytes=0-0" || fake.served != 1 {
		t.Fatalf("warm-up fetched ranges=%v served=%d", fake.ranges, fake.served)
	}
}
