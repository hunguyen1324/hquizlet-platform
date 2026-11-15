package http

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestPublicVideoProviders(t *testing.T) {
	for _, tc := range []struct{ url, source string }{
		{"https://youtu.be/M7lc1UVf-VE", "youtube"},
		{"https://www.tiktok.com/@creator/video/6718335390845095173", "tiktok"},
		{"https://www.facebook.com/creator/videos/123456/", "facebook"},
		{"https://www.facebook.com/reel/123456", "facebook"},
		{"https://cdn.example.org/lesson.mp4?token=abc", "direct"},
		{"https://youtube.com.evil.test/watch?v=M7lc1UVf-VE", ""},
		{"javascript:alert(1)", ""}, {"https://www.tiktok.com@evil.test/video/1234567890", ""},
		{"https://vm.tiktok.com/short/", ""}, {"https://example.org/page", ""},
	} {
		t.Run(tc.url, func(t *testing.T) {
			source, embed, _ := publicVideoSource(tc.url)
			if source != tc.source {
				t.Fatalf("got %q expected %q", source, tc.source)
			}
			if source != "" && !strings.HasPrefix(embed, "https://") {
				t.Fatal("unsafe embed", embed)
			}
		})
	}
}
func mediaRow(owner int64, visibility string) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "owner", "kind", "title", "body", "source", "url", "embed", "thumb", "visibility", "viewers", "created"}).AddRow(int64(7), owner, "video", "Lesson", "Description", "youtube", "https://youtu.be/M7lc1UVf-VE", "https://www.youtube.com/embed/M7lc1UVf-VE", "", visibility, `[2,3]`, time.Now())
}
func TestMediaAuthorization(t *testing.T) {
	for _, tc := range []struct {
		name, method, path string
		owner              int64
		missing            bool
		status             int
	}{
		{"cannot delete someone else's public video", "DELETE", "/v1/media/7", 1, false, 403},
		{"cannot read revoked video", "GET", "/v1/media/7", 1, true, 404},
		{"viewer can read without seeing other viewer IDs", "GET", "/v1/media/7", 1, false, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, e := sqlmock.New()
			if e != nil {
				t.Fatal(e)
			}
			defer db.Close()
			h := &Handler{db: db}
			q := mock.ExpectQuery(`SELECT .* FROM media_items m WHERE .* AND m.id=\$3`).WithArgs(int64(2), "[2]", int64(7))
			if tc.missing {
				q.WillReturnError(sql.ErrNoRows)
			} else {
				q.WillReturnRows(mediaRow(tc.owner, "selected"))
			}
			r := httptest.NewRequest(tc.method, tc.path, nil)
			r.Header.Set("X-User-ID", "2")
			w := httptest.NewRecorder()
			h.mediaRouter(w, r)
			if w.Code != tc.status {
				t.Fatalf("%d: %s", w.Code, w.Body.String())
			}
			if tc.status == 200 {
				var m mediaItem
				if json.Unmarshal(w.Body.Bytes(), &m) != nil || len(m.ViewerIDs) != 0 {
					t.Fatal("viewer IDs disclosed", w.Body.String())
				}
			}
			if e = mock.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestMediaRequiresLogin(t *testing.T) {
	h := &Handler{}
	w := httptest.NewRecorder()
	h.mediaRouter(w, httptest.NewRequest("GET", "/v1/media", nil))
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
}
func TestScopedMangaFailsClosedWithoutAccessService(t *testing.T) {
	h := &Handler{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/v1/manga?classId=12", nil)
	r.Header.Set("X-User-ID", "1")
	h.mangaRouter(w, r)
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
}
func TestPublicUploadStoresSelectedAudienceAtomically(t *testing.T) {
	db, mock, e := sqlmock.New()
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	h := &Handler{db: db, videoStore: &memoryVideoStore{}, videoSecret: bytes.Repeat([]byte{1}, 32), videoMaxBytes: 1024}
	mock.ExpectQuery("INSERT INTO media_items").WithArgs(int64(1), "Lesson", sqlmock.AnyArg(), "video/mp4", "Notes", "selected", "[2]").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	form.WriteField("metadata", `{"title":"Lesson","body":"Notes","visibility":"selected","viewerIds":[2]}`)
	part, e := form.CreateFormFile("file", "lesson.mp4")
	if e != nil {
		t.Fatal(e)
	}
	part.Write(mp4Fixture())
	form.Close()
	r := httptest.NewRequest("POST", "/v1/media/upload", &body)
	r.Header.Set("X-User-ID", "1")
	r.Header.Set("Content-Type", form.FormDataContentType())
	w := httptest.NewRecorder()
	h.mediaRouter(w, r)
	if w.Code != 201 {
		t.Fatalf("%d: %s", w.Code, w.Body.String())
	}
	if e = mock.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}

func TestCommunityStreamRejectsClassTicketsAndRechecksAudience(t *testing.T) {
	secret := bytes.Repeat([]byte{3}, 32)
	h := &Handler{videoStore: &memoryVideoStore{}, videoSecret: secret}
	ticket := videoTicket{UserID: 2, ClassID: 1, VideoID: 7, Expires: time.Now().Add(time.Minute).Unix()}
	w := httptest.NewRecorder()
	h.streamPublicMedia(w, httptest.NewRequest("GET", "/v1/media-stream?ticket="+signVideoTicket(secret, ticket), nil))
	if w.Code != 401 {
		t.Fatalf("class ticket accepted by community stream: %d", w.Code)
	}
	db, mock, e := sqlmock.New()
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	h.db = db
	key := sha256.Sum256(append(append([]byte{}, secret...), []byte("public-media")...))
	mock.ExpectQuery("SELECT storage_key,content_type FROM media_items").WithArgs(int64(2), "[2]", int64(7)).WillReturnError(sql.ErrNoRows)
	w = httptest.NewRecorder()
	h.streamPublicMedia(w, httptest.NewRequest("GET", "/v1/media-stream?ticket="+signVideoTicket(key[:], ticket), nil))
	if w.Code != 404 {
		t.Fatalf("revoked audience accepted: %d", w.Code)
	}
	if e = mock.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
func TestPlaylistAppendIsAtomicAndChecksVideoAccess(t *testing.T) {
	for _, allowed := range []bool{true, false} {
		t.Run(strconv.FormatBool(allowed), func(t *testing.T) {
			db, mock, e := sqlmock.New()
			if e != nil {
				t.Fatal(e)
			}
			defer db.Close()
			h := &Handler{db: db}
			mock.ExpectQuery("SELECT .* FROM media_items m WHERE").WithArgs(int64(2), "[2]", int64(7)).WillReturnRows(sqlmock.NewRows([]string{"id", "owner", "kind", "title", "body", "source", "url", "embed", "thumb", "visibility", "viewers", "created"}).AddRow(7, 2, "playlist", "My playlist", "", "", "", "", "", "public", "[]", time.Now()))
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT id FROM media_items").WithArgs(int64(7), int64(2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
			mock.ExpectQuery("SELECT EXISTS").WithArgs(int64(7), int64(9)).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
			n := int64(0)
			want := 422
			if allowed {
				n = 1
				want = 204
			}
			mock.ExpectExec("INSERT INTO media_playlist_items").WithArgs(int64(2), "[2]", int64(7), int64(9)).WillReturnResult(sqlmock.NewResult(0, n))
			if allowed {
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}
			r := httptest.NewRequest("POST", "/v1/media/7/items/9", nil)
			r.Header.Set("X-User-ID", "2")
			w := httptest.NewRecorder()
			h.mediaRouter(w, r)
			if w.Code != want {
				t.Fatalf("%d: %s", w.Code, w.Body.String())
			}
			if e = mock.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
