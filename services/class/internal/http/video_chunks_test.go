package http

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

type chunkTestStore struct {
	memoryVideoStore
	written bool
}

func (s *chunkTestStore) CheckSpace(context.Context, int64) error { return nil }
func (s *chunkTestStore) WriteChunk(_ context.Context, _ string, _ int64, r io.Reader) error {
	s.written = true
	_, err := io.Copy(io.Discard, r)
	return err
}
func (s *chunkTestStore) FinishUpload(context.Context, string, int64) error { return nil }
func (s *chunkTestStore) AbortUpload(context.Context, string) error         { return nil }

func TestChunkedVideoRejectsUnauthorizedAndOversize(t *testing.T) {
	for _, tt := range []struct {
		role, body string
		want       int
	}{{"student", `{}`, 403}, {"owner", `{"title":"large","size":1073741825,"contentType":"video/mp4","audience":"all"}`, 413}} {
		t.Run(tt.role, func(t *testing.T) {
			h, m, _ := privateTestHandler(t, tt.role)
			h.videoStore = &chunkTestStore{}
			h.videoMaxBytes = maxChunkedVideoBytes
			r := httptest.NewRequest("POST", "/v1/classes/1/videos/uploads", strings.NewReader(tt.body))
			r.Header.Set("X-User-ID", "2")
			w := httptest.NewRecorder()
			h.videoRouter(w, r, 1, []string{"uploads"})
			if w.Code != tt.want {
				t.Fatalf("status=%d body=%s", w.Code, w.Body)
			}
			if err := m.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestChunkRejectsWrongOffsetAndMissingParts(t *testing.T) {
	for _, complete := range []bool{false, true} {
		h, m, _ := privateTestHandler(t, "owner")
		s := &chunkTestStore{}
		h.videoStore = s
		m.ExpectBegin()
		m.ExpectQuery("SELECT storage_key,title").WithArgs(strings.Repeat("a", 32), int64(1), int64(2)).WillReturnRows(sqlmock.NewRows([]string{"key", "title", "audience", "viewers", "ct", "size", "received", "video"}).AddRow(strings.Repeat("a", 32)+".mp4", "lll", "all", "[]", "video/mp4", 100, 0, nil))
		m.ExpectRollback()
		method := "PUT"
		parts := []string{"uploads", strings.Repeat("a", 32)}
		if complete {
			method = "POST"
			parts = append(parts, "complete")
		}
		r := httptest.NewRequest(method, "/x?offset=99", strings.NewReader("bad"))
		r.Header.Set("X-User-ID", "2")
		w := httptest.NewRecorder()
		h.chunkedVideo(w, r, 1, 1, parts)
		if w.Code != http.StatusConflict || s.written {
			t.Fatalf("status=%d wrote=%v", w.Code, s.written)
		}
		if err := m.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCompleteRetryDoesNotCreateDuplicateVideo(t *testing.T) {
	h, m, _ := privateTestHandler(t, "owner")
	h.videoStore = &chunkTestStore{}
	m.ExpectBegin()
	m.ExpectQuery("SELECT storage_key,title").WithArgs(strings.Repeat("a", 32), int64(1), int64(2)).WillReturnRows(sqlmock.NewRows([]string{"key", "title", "audience", "viewers", "ct", "size", "received", "video"}).AddRow(strings.Repeat("a", 32)+".mp4", "lll", "all", "[]", "video/mp4", 100, 100, 7))
	m.ExpectCommit()
	r := httptest.NewRequest("POST", "/x", nil)
	r.Header.Set("X-User-ID", "2")
	w := httptest.NewRecorder()
	h.chunkedVideo(w, r, 1, 1, []string{"uploads", strings.Repeat("a", 32), "complete"})
	if w.Code != 202 || !strings.Contains(w.Body.String(), `"id":7`) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
