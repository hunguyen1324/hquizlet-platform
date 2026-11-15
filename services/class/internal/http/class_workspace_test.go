package http

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/service"
)

func accessTestHandler(t *testing.T, role string) (*Handler, sqlmock.Sqlmock) {
	h, mock, _ := privateTestHandler(t, role)
	h.access = &service.ClassAccess{DB: h.db, Classes: videoClassStore{}, Members: videoMemberStore{role: role}}
	return h, mock
}
func expectLegacyAccess(mock sqlmock.Sqlmock, uid int64) {
	mock.ExpectQuery("SELECT role_keys FROM class_member_access").WithArgs(int64(1), uid).WillReturnError(sql.ErrNoRows)
}
func TestCustomRolesCannotModifyManagedRolesOrOwner(t *testing.T) {
	for _, tc := range []struct {
		path   string
		parts  []string
		body   string
		status int
	}{
		{"roles/teacher", []string{"roles", "teacher"}, `{"name":"oops"}`, 422},
		{"members/1", []string{"members", "1"}, `{"roleKeys":[]}`, 403},
	} {
		h, mock := accessTestHandler(t, "owner")
		mock.ExpectBegin()
		mock.ExpectQuery("SELECT owner_user_id FROM classes").WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"owner"}).AddRow(1))
		mock.ExpectRollback()
		r := httptest.NewRequest("PUT", "/v1/classes/1/access/"+tc.path, strings.NewReader(tc.body))
		r.Header.Set("X-User-ID", "1")
		w := httptest.NewRecorder()
		h.accessRouter(w, r, 1, tc.parts)
		if w.Code != tc.status {
			t.Fatal(w.Code, w.Body.String())
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	}
}
func TestAssignmentOfRoleFromAnotherClassFails(t *testing.T) {
	h, mock := accessTestHandler(t, "owner")
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT owner_user_id FROM classes").WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"owner"}).AddRow(1))
	mock.ExpectQuery("SELECT key FROM class_role_definitions").WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"key"}))
	mock.ExpectRollback()
	r := httptest.NewRequest("PUT", "/v1/classes/1/access/members/2", strings.NewReader(`{"roleKeys":["custom-other-class"]}`))
	r.Header.Set("X-User-ID", "1")
	w := httptest.NewRecorder()
	h.accessRouter(w, r, 1, []string{"members", "2"})
	if w.Code != 422 {
		t.Fatal(w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func assignmentRequest(t *testing.T, method string, input assignmentInput, filename string, data []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	raw, _ := json.Marshal(input)
	writer.WriteField("metadata", string(raw))
	if filename != "" {
		f, err := writer.CreateFormFile("files", filename)
		if err != nil {
			t.Fatal(err)
		}
		f.Write(data)
	}
	writer.Close()
	r := httptest.NewRequest(method, "/v1/classes/1/assignments", &body)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	r.Header.Set("X-User-ID", "1")
	return r
}
func TestAssignmentAndFilesCommitTogether(t *testing.T) {
	for _, fail := range []bool{false, true} {
		h, mock := accessTestHandler(t, "owner")
		now := time.Now()
		data := []byte("%PDF-1.7\nfixture")
		mock.ExpectBegin()
		mock.ExpectQuery("INSERT INTO class_assignments").WithArgs(int64(1), "Ôn tập", "Hãy làm câu 1", nil, "published", int64(1)).WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).AddRow(9, now, now))
		mock.ExpectQuery("SELECT count\\(\\*\\),COALESCE").WithArgs(int64(9)).WillReturnRows(sqlmock.NewRows([]string{"count", "bytes"}).AddRow(0, 0))
		insert := mock.ExpectExec("INSERT INTO class_assignment_files").WithArgs(int64(9), "Đề.pdf", "application/pdf", data)
		if fail {
			insert.WillReturnError(errors.New("storage unavailable"))
			mock.ExpectRollback()
		} else {
			insert.WillReturnResult(sqlmock.NewResult(3, 1))
			mock.ExpectQuery("SELECT id,name,content_type,octet_length").WithArgs(int64(9)).WillReturnRows(sqlmock.NewRows([]string{"id", "name", "ct", "bytes"}).AddRow(3, "Đề.pdf", "application/pdf", len(data)))
			mock.ExpectCommit()
		}
		r := assignmentRequest(t, "POST", assignmentInput{Title: "Ôn tập", Instructions: "Hãy làm câu 1", Status: "published"}, "Đề.pdf", data)
		w := httptest.NewRecorder()
		h.assignmentRouter(w, r, 1, nil)
		want := 201
		if fail {
			want = 500
		}
		if w.Code != want {
			t.Fatal(fail, w.Code, w.Body.String())
		}
		if !fail && !strings.Contains(w.Body.String(), "Đề.pdf") {
			t.Fatal(w.Body.String())
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	}
}
func TestAssignmentRejectsInvalidFileBeforeWriting(t *testing.T) {
	h, mock := accessTestHandler(t, "owner")
	r := assignmentRequest(t, "POST", assignmentInput{Title: "Lesson", Status: "draft"}, "wrong.pdf", []byte("<html>"))
	w := httptest.NewRecorder()
	h.assignmentRouter(w, r, 1, nil)
	if w.Code != 422 {
		t.Fatal(w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestLearnerCannotWriteAssignmentsAndCannotReadDraftFiles(t *testing.T) {
	h, mock := accessTestHandler(t, "student")
	expectLegacyAccess(mock, 2)
	expectLegacyAccess(mock, 2)
	r := assignmentRequest(t, "POST", assignmentInput{Title: "Not allowed", Status: "draft"}, "", nil)
	r.Header.Set("X-User-ID", "2")
	w := httptest.NewRecorder()
	h.assignmentRouter(w, r, 1, nil)
	if w.Code != 403 {
		t.Fatal(w.Code, w.Body.String())
	}
	expectLegacyAccess(mock, 2)
	mock.ExpectQuery("SELECT f.name,f.content_type,f.data").WithArgs(int64(1), int64(9), int64(3), true, false, int64(2), false).WillReturnError(sql.ErrNoRows)
	r = httptest.NewRequest("GET", "/v1/classes/1/assignments/9/files/3", nil)
	r.Header.Set("X-User-ID", "2")
	w = httptest.NewRecorder()
	h.assignmentRouter(w, r, 1, []string{"9", "files", "3"})
	if w.Code != 404 {
		t.Fatal(w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestPublishedAssignmentFileDownloadIsPrivateAndClassScoped(t *testing.T) {
	h, mock := accessTestHandler(t, "student")
	expectLegacyAccess(mock, 2)
	data := []byte("%PDF-1.7\nprivate")
	mock.ExpectQuery("SELECT f.name,f.content_type,f.data").WithArgs(int64(1), int64(9), int64(3), true, false, int64(2), false).WillReturnRows(sqlmock.NewRows([]string{"name", "ct", "data"}).AddRow("Bài tập.pdf", "application/pdf", data))
	r := httptest.NewRequest("GET", "/v1/classes/1/assignments/9/files/3", nil)
	r.Header.Set("X-User-ID", "2")
	w := httptest.NewRecorder()
	h.assignmentRouter(w, r, 1, []string{"9", "files", "3"})
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), data) || w.Header().Get("Cache-Control") != "private, no-store" || !strings.HasPrefix(w.Header().Get("Content-Disposition"), "attachment;") {
		t.Fatal(w.Code, w.Header(), w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestAssignmentDraftListIsFilteredByPermission(t *testing.T) {
	h, mock := accessTestHandler(t, "student")
	expectLegacyAccess(mock, 2)
	mock.ExpectQuery("SELECT a.id,a.title,a.instructions").WithArgs(int64(1), true, false, int64(2), false).WillReturnRows(sqlmock.NewRows([]string{"id", "title", "instructions", "due", "status", "created_by", "created_at", "updated_at", "files"}))
	r := httptest.NewRequest("GET", "/v1/classes/1/assignments", nil)
	r.Header.Set("X-User-ID", "2")
	w := httptest.NewRecorder()
	h.assignmentRouter(w, r, 1, nil)
	if w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatal(w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestRevokedTeacherCannotWriteVideosDocumentsOrCreateRoles(t *testing.T) {
	for _, kind := range []string{"videos", "documents", "roles"} {
		h, mock := accessTestHandler(t, "teacher")
		mock.ExpectQuery("SELECT role_keys").WithArgs(int64(1), int64(2)).WillReturnRows(sqlmock.NewRows([]string{"keys"}).AddRow(`["student"]`))
		mock.ExpectQuery("SELECT key,name,description,allow_actions,deny_actions").WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"key", "name", "description", "allow", "deny"}))
		r := httptest.NewRequest("POST", "/v1/classes/1/"+kind, strings.NewReader(`{}`))
		r.Header.Set("X-User-ID", "2")
		w := httptest.NewRecorder()
		switch kind {
		case "videos":
			h.videoRouter(w, r, 1, nil)
		case "documents":
			h.documentRouter(w, r, 1, nil)
		case "roles":
			h.accessRouter(w, r, 1, []string{"roles"})
		}
		if w.Code != 403 {
			t.Fatal(kind, w.Code, w.Body.String())
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(kind, err)
		}
	}
}
