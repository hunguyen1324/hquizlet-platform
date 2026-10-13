package http

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/service"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/videostorage"
)

const testVideoSecret = "01234567890123456789012345678901"

type memoryVideoStore struct {
	body            []byte
	opened, deleted bool
	putErr          error
}

func (s *memoryVideoStore) Put(_ context.Context, _ string, r io.Reader) error {
	if s.putErr != nil {
		return s.putErr
	}
	var err error
	s.body, err = io.ReadAll(r)
	return err
}
func (s *memoryVideoStore) Open(context.Context, string) (videostorage.File, error) {
	s.opened = true
	return &memoryVideoFile{Reader: bytes.NewReader(s.body)}, nil
}
func (s *memoryVideoStore) Delete(context.Context, string) error { s.deleted = true; return nil }

type memoryVideoFile struct{ *bytes.Reader }

func (f *memoryVideoFile) Close() error               { return nil }
func (f *memoryVideoFile) Stat() (os.FileInfo, error) { return videoFileInfo{f.Size()}, nil }

type videoFileInfo struct{ size int64 }

func (v videoFileInfo) Name() string       { return "video.mp4" }
func (v videoFileInfo) Size() int64        { return v.size }
func (v videoFileInfo) Mode() os.FileMode  { return 0600 }
func (v videoFileInfo) ModTime() time.Time { return time.Unix(1, 0) }
func (v videoFileInfo) IsDir() bool        { return false }
func (v videoFileInfo) Sys() any           { return nil }

func privateTestHandler(t *testing.T, role string) (*Handler, sqlmock.Sqlmock, *memoryVideoStore) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	classes, members := videoClassStore{}, videoMemberStore{role: role}
	h := &Handler{db: db, classes: service.NewClassService(classes, members), members: service.NewMemberService(classes, members)}
	store := &memoryVideoStore{body: []byte("0123456789")}
	h.ConfigureVideos(store, testVideoSecret, 1024)
	h.hlsSecret = []byte(testVideoSecret)
	return h, mock, store
}

func TestVideoTicketRejectsTamperingAndExpiry(t *testing.T) {
	now := time.Now()
	ticket := videoTicket{2, 1, 3, now.Add(time.Minute).Unix()}
	raw := signVideoTicket([]byte(testVideoSecret), ticket)
	if _, err := verifyVideoTicket([]byte(testVideoSecret), raw, now); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{raw + "x", "bad", signVideoTicket([]byte(testVideoSecret), videoTicket{2, 1, 3, now.Unix()}), signVideoTicket([]byte(testVideoSecret), videoTicket{2, 1, 3, now.Add(3 * time.Hour).Unix()})} {
		if _, err := verifyVideoTicket([]byte(testVideoSecret), value, now); err == nil {
			t.Fatal("accepted invalid ticket")
		}
	}
}

func TestPrivateVideoRangeAndRevocation(t *testing.T) {
	for _, tc := range []struct {
		name, role, method, rangeHeader string
		row                             bool
		status                          int
		body                            string
	}{
		{"range", "student", "GET", "bytes=2-5", true, 206, "2345"},
		{"suffix", "student", "GET", "bytes=-3", true, 206, "789"},
		{"invalid range", "student", "GET", "bytes=99-100", true, 416, ""},
		{"head", "student", "HEAD", "", true, 200, ""},
		{"video permission revoked", "student", "GET", "", false, 404, ""},
		{"class membership revoked", "", "GET", "", false, 403, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, mock, store := privateTestHandler(t, tc.role)
			if tc.role != "" {
				query := mock.ExpectQuery("SELECT storage_key,content_type FROM class_videos").WithArgs(int64(1), int64(3), false, "[2]")
				if tc.row {
					query.WillReturnRows(sqlmock.NewRows([]string{"storage_key", "content_type"}).AddRow(strings.Repeat("a", 32)+".mp4", "video/mp4"))
				} else {
					query.WillReturnError(sql.ErrNoRows)
				}
			}
			ticket := signVideoTicket(h.videoSecret, videoTicket{2, 1, 3, time.Now().Add(time.Minute).Unix()})
			r := httptest.NewRequest(tc.method, "/v1/class-video-stream?ticket="+ticket, nil)
			r.Header.Set("Range", tc.rangeHeader)
			r.Header.Set("X-User-ID", "1") // spoofed owner must be ignored
			w := httptest.NewRecorder()
			h.streamPrivateVideo(w, r)
			if w.Code != tc.status {
				t.Fatalf("status %d body %s", w.Code, w.Body.String())
			}
			if tc.body != "" && w.Body.String() != tc.body {
				t.Fatalf("body %q", w.Body.String())
			}
			if tc.method == "HEAD" && w.Body.Len() != 0 {
				t.Fatal("HEAD returned a body")
			}
			if tc.status >= 400 && tc.status != 416 && store.opened {
				t.Fatal("opened storage for unauthorized viewer")
			}
			if tc.status == 206 && w.Header().Get("Content-Range") == "" {
				t.Fatal("missing range metadata")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPrivatePlaybackTicketRequiresVideoPermission(t *testing.T) {
	h, mock, _ := privateTestHandler(t, "student")
	mock.ExpectQuery("SELECT storage_key,content_type").WithArgs(int64(1), int64(3), false, "[2]").WillReturnError(sql.ErrNoRows)
	r := httptest.NewRequest("POST", "/v1/classes/1/videos/3/playback", nil)
	r.Header.Set("X-User-ID", "2")
	w := httptest.NewRecorder()
	h.videoRouter(w, r, 1, []string{"3", "playback"})
	if w.Code != 404 {
		t.Fatalf("got %d", w.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func makeVideoUpload(t *testing.T, data []byte) (*http.Request, *bytes.Buffer) {
	t.Helper()
	body := new(bytes.Buffer)
	mw := multipart.NewWriter(body)
	part, err := mw.CreateFormFile("file", "lesson.mp4")
	if err != nil {
		t.Fatal(err)
	}
	part.Write(data)
	mw.Close()
	r := httptest.NewRequest("POST", "/v1/classes/1/videos/upload?title=Lesson&audience=all", body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	r.Header.Set("X-User-ID", "1")
	return r, body
}

func mp4Fixture() []byte {
	data := make([]byte, 512)
	copy(data, []byte{0, 0, 0, 24, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm', 0, 0, 0, 0, 'i', 's', 'o', 'm', 'm', 'p', '4', '2'})
	return data
}

func TestPrivateUploadValidationAndRollback(t *testing.T) {
	for _, tc := range []struct {
		name    string
		body    []byte
		max     int64
		putErr  error
		dbErr   bool
		status  int
		deleted bool
	}{
		{"success", mp4Fixture(), 1024, nil, false, 201, false},
		{"fake extension", []byte("not a video"), 1024, nil, false, 415, false},
		{"oversize", mp4Fixture(), 100, nil, false, 413, false},
		{"storage failure", mp4Fixture(), 1024, errors.New("disk full"), false, 502, false},
		{"database failure cleans upload", mp4Fixture(), 1024, nil, true, 500, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.status == 201 || tc.dbErr {
				tc.body = realVideoFixture(t)
				tc.max = 1024 * 1024
			}
			h, mock, store := privateTestHandler(t, "owner")
			h.videoMaxBytes = tc.max
			store.putErr = tc.putErr
			if tc.status == 201 || tc.dbErr {
				q := mock.ExpectQuery("INSERT INTO class_videos").WithArgs(int64(1), "Lesson", "all", "[]", sqlmock.AnyArg(), hlsContentType, int64(len(tc.body)))
				if tc.dbErr {
					q.WillReturnError(errors.New("write failed"))
				} else {
					q.WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))
				}
			}
			r, _ := makeVideoUpload(t, tc.body)
			w := httptest.NewRecorder()
			h.videoRouter(w, r, 1, []string{"upload"})
			if w.Code != tc.status {
				t.Fatalf("got %d body %s", w.Code, w.Body.String())
			}
			if store.deleted != tc.deleted {
				t.Fatalf("cleanup %v", store.deleted)
			}
			if tc.status == 201 {
				var v classVideo
				if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil || v.Source != "sftp" || v.SizeBytes != int64(len(tc.body)) {
					t.Fatal("invalid metadata")
				}
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func realVideoFixture(t *testing.T) []byte {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("FFmpeg integration test requires ffmpeg")
	}
	file := filepath.Join(t.TempDir(), "test.mp4")
	cmd := exec.Command("ffmpeg", "-nostdin", "-loglevel", "error", "-f", "lavfi", "-i", "color=c=blue:s=160x120:r=10", "-t", "1", "-c:v", "libx264", "-pix_fmt", "yuv420p", file)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v %s", err, out)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestStudentCannotUploadBeforeReadingBody(t *testing.T) {
	h, mock, _ := privateTestHandler(t, "student")
	r, _ := makeVideoUpload(t, mp4Fixture())
	r.Header.Set("X-User-ID", "2")
	w := httptest.NewRecorder()
	h.videoRouter(w, r, 1, []string{"upload"})
	if w.Code != 403 {
		t.Fatalf("got %d", w.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestVideoCleanupRetainsFailedDeletion(t *testing.T) {
	h, mock, store := privateTestHandler(t, "owner")
	key := strings.Repeat("a", 32) + ".mp4"
	mock.ExpectQuery("SELECT storage_key FROM class_video_deletions").WillReturnRows(sqlmock.NewRows([]string{"storage_key"}).AddRow(key))
	mock.ExpectExec("DELETE FROM class_video_deletions").WithArgs(key).WillReturnError(errors.New("database unavailable"))
	if err := h.cleanupVideos(context.Background()); err == nil {
		t.Fatal("lost cleanup failure")
	}
	if !store.deleted {
		t.Fatal("did not delete remote file")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	// Retry succeeds when the remote file is already absent (idempotent Delete).
	mock.ExpectQuery("SELECT storage_key FROM class_video_deletions").WillReturnRows(sqlmock.NewRows([]string{"storage_key"}).AddRow(key))
	mock.ExpectExec("DELETE FROM class_video_deletions").WithArgs(key).WillReturnResult(sqlmock.NewResult(0, 1))
	if err := h.cleanupVideos(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
