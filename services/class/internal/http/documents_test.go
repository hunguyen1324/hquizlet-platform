package http

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestDocumentsEnforceMembershipAndStaffWrites(t *testing.T) {
	for _, tc := range []struct {
		role, uid, method string
		status            int
	}{
		{"", "", "GET", 401}, {"", "2", "GET", 403},
		{"student", "2", "POST", 403}, {"student", "2", "DELETE", 403},
	} {
		h, mock, _ := privateTestHandler(t, tc.role)
		r := httptest.NewRequest(tc.method, "/v1/classes/1/documents", strings.NewReader("ignored"))
		r.Header.Set("X-User-ID", tc.uid)
		w := httptest.NewRecorder()
		h.documentRouter(w, r, 1, nil)
		if w.Code != tc.status {
			t.Fatalf("%+v: %d %s", tc, w.Code, w.Body.String())
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDocumentUploadAndClassScopedDownload(t *testing.T) {
	h, mock, _ := privateTestHandler(t, "teacher")
	data := []byte("%PDF-1.7\nexample")
	mock.ExpectQuery("INSERT INTO class_documents").WithArgs(int64(1), "Bài học.pdf", "application/pdf", data).WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(7, time.Now()))
	r := httptest.NewRequest("POST", "/v1/classes/1/documents?name=B%C3%A0i%20h%E1%BB%8Dc.pdf", bytes.NewReader(data))
	r.Header.Set("X-User-ID", "2")
	w := httptest.NewRecorder()
	h.documentRouter(w, r, 1, nil)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	mock.ExpectQuery("SELECT name,content_type,data FROM class_documents WHERE class_id=\\$1 AND id=\\$2").WithArgs(int64(1), int64(7)).WillReturnRows(sqlmock.NewRows([]string{"name", "content_type", "data"}).AddRow("Bài học.pdf", "application/pdf", data))
	r = httptest.NewRequest("GET", "/v1/classes/1/documents/7", nil)
	r.Header.Set("X-User-ID", "2")
	w = httptest.NewRecorder()
	h.documentRouter(w, r, 1, []string{"7"})
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), data) || !strings.HasPrefix(w.Header().Get("Content-Disposition"), "attachment;") || w.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal(w.Code, w.Header(), w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDocumentRejectsUnsupportedEmptyAndOversizeFiles(t *testing.T) {
	for _, tc := range []struct {
		name   string
		data   []byte
		status int
	}{
		{"file.exe", []byte("MZ"), 422}, {"file.pdf", nil, 422}, {"file.pdf", []byte("<html>"), 422},
		{"file.pdf", bytes.Repeat([]byte{'x'}, maxDocumentBytes+1), 413},
	} {
		h, mock, _ := privateTestHandler(t, "teacher")
		r := httptest.NewRequest("POST", "/v1/classes/1/documents?name="+tc.name, bytes.NewReader(tc.data))
		r.Header.Set("X-User-ID", "2")
		w := httptest.NewRecorder()
		h.documentRouter(w, r, 1, nil)
		if w.Code != tc.status {
			t.Fatal(tc.name, w.Code, w.Body.String())
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCategoryUpdatesRequireStaffAndAreClassScoped(t *testing.T) {
	h, mock, _ := privateTestHandler(t, "teacher")
	mock.ExpectExec("UPDATE class_videos SET category").WithArgs("Giao tiếp", int64(1), int64(3)).WillReturnResult(sqlmock.NewResult(0, 1))
	r := httptest.NewRequest("PUT", "/v1/classes/1/videos/3/category", strings.NewReader(`{"category":" Giao tiếp "}`))
	r.Header.Set("X-User-ID", "2")
	w := httptest.NewRecorder()
	h.videoRouter(w, r, 1, []string{"3", "category"})
	if w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	h, mock, _ = privateTestHandler(t, "student")
	r = httptest.NewRequest("PUT", "/v1/classes/1/videos/3/category", strings.NewReader(`{"category":"X"}`))
	r.Header.Set("X-User-ID", "2")
	w = httptest.NewRecorder()
	h.videoRouter(w, r, 1, []string{"3", "category"})
	if w.Code != 403 {
		t.Fatal(w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
