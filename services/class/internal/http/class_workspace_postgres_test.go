package http

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/repository"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/service"
	"github.com/hunguyen1324/hquizlet-platform/services/class/migrations"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Set CLASS_TEST_DATABASE_URL to an isolated PostgreSQL instance. Each run creates
// its own schema and removes only that schema, never existing application tables.
func TestClassWorkspacePostgres(t *testing.T) {
	dsn := os.Getenv("CLASS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("CLASS_TEST_DATABASE_URL is not configured")
	}
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })
	schema := fmt.Sprintf("class_workspace_%d", time.Now().UnixNano())
	if _, err = admin.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); err != nil {
			t.Error(err)
		}
	})
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	db, err := sql.Open("pgx", dsn+separator+"search_path="+schema)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for i := 0; i < 2; i++ {
		if err = migrations.Run(db); err != nil {
			t.Fatal("migration", err)
		}
	}
	classes := repository.NewClassRepository(db)
	members := repository.NewMemberRepository(db)
	sets := repository.NewClassStudySetRepository(db)
	c, err := classes.Create(context.Background(), 1, model.CreateClassInput{Name: "QA class", MaxMembers: 100})
	if err != nil {
		t.Fatal(err)
	}
	other, err := classes.Create(context.Background(), 99, model.CreateClassInput{Name: "Other class", MaxMembers: 100})
	if err != nil {
		t.Fatal(err)
	}
	for uid, role := range map[int64]string{1: "owner", 2: "teacher", 3: "student"} {
		if _, err = members.Add(context.Background(), c.ID, uid, role); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = members.Add(context.Background(), other.ID, 3, "student"); err != nil {
		t.Fatal(err)
	}
	cs := service.NewClassService(classes, members)
	ms := service.NewMemberService(classes, members)
	ss := service.NewClassStudySetService(classes, members, sets, nil)
	access := service.ConfigureClassAccess(db, cs, ms, ss)
	h := New(cs, ms, ss, service.NewActivityService(repository.NewActivityRepository(db), nil), db).WithClassAccess(access)
	t.Run("manga library", func(t *testing.T) { testMangaWorkspace(t, h, c.ID) })
	call := func(method, path, body, uid string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, fmt.Sprintf("/v1/classes/%d/%s", c.ID, path), strings.NewReader(body))
		r.Header.Set("X-User-ID", uid)
		w := httptest.NewRecorder()
		h.classRouter(w, r)
		return w
	}
	assertStatus := func(w *httptest.ResponseRecorder, want int) {
		t.Helper()
		if w.Code != want {
			t.Fatalf("expected %d, got %d: %s", want, w.Code, w.Body.String())
		}
	}
	w := call("POST", "access/roles", `{"name":"Bài tập","description":"Quản lý bài tập","allow":["class:Read","class-assignment:Read","class-assignment:Create","class-assignment:Update","class-assignment:Publish","class-content:ViewVideo","class-video:Update"],"deny":["class-video:Delete"]}`, "1")
	assertStatus(w, 200)
	var role service.ClassRole
	if err = json.Unmarshal(w.Body.Bytes(), &role); err != nil {
		t.Fatal(err)
	}
	assertStatus(call("PUT", "access/members/2", fmt.Sprintf(`{"roleKeys":[%q]}`, role.Key), "1"), 200)
	assertStatus(call("DELETE", "access/roles/"+role.Key, "", "1"), 409)
	assertStatus(call("GET", "access", "", "1"), 200)
	assertStatus(call("PUT", "access/roles/"+role.Key, fmt.Sprintf(`{"name":"Bài tập cập nhật","description":"Quản lý bài tập","allow":%s,"deny":%s}`, mustJSON(role.Allow), mustJSON(role.Deny)), "1"), 200)
	assertStatus(call("POST", "documents?name=test.pdf", "%PDF-1.7", "2"), 403)
	assertStatus(call("DELETE", "videos/1", "", "2"), 403)

	var videoID int64
	if err = db.QueryRow(`INSERT INTO class_videos(class_id,title,youtube_id,audience,viewer_ids) VALUES($1,'Original','dQw4w9WgXcQ','selected','[2,3]'::jsonb) RETURNING id`, c.ID).Scan(&videoID); err != nil {
		t.Fatal(err)
	}
	w = call("GET", "videos", "", "2")
	assertStatus(w, 200)
	var videos []classVideo
	if err = json.Unmarshal(w.Body.Bytes(), &videos); err != nil {
		t.Fatal(err)
	}
	if len(videos) != 1 || len(videos[0].ViewerIDs) != 0 {
		t.Fatal("viewer list exposed", w.Body.String())
	}
	assertStatus(call("PUT", fmt.Sprintf("videos/%d", videoID), `{"title":"Edited title","url":"https://youtu.be/dQw4w9WgXcQ"}`, "2"), 200)
	// Persist subtitles against the real migration and enforce current video audience.
	subtitlePath := fmt.Sprintf("videos/%d/subtitles", videoID)
	subtitleBody := `{"kind":"original","language":"ja","label":"Lesson","cues":[{"start":0,"end":3,"text":"<b>こんにちは</b>"}]}`
	assertStatus(call("PUT", subtitlePath, subtitleBody, "2"), 200)
	w = call("GET", subtitlePath, "", "3")
	assertStatus(w, 200)
	if !strings.Contains(w.Body.String(), `"source":"uploaded"`) || strings.Contains(w.Body.String(), "<b>") {
		t.Fatal("saved subtitle not sanitized", w.Body.String())
	}
	assertStatus(call("PUT", subtitlePath, subtitleBody, "3"), 403)
	if _, err = db.Exec(`UPDATE class_videos SET viewer_ids='[2]'::jsonb WHERE id=$1`, videoID); err != nil {
		t.Fatal(err)
	}
	assertStatus(call("GET", subtitlePath, "", "3"), 404)
	assertStatus(call("DELETE", subtitlePath+"?kind=original", "", "2"), 204)
	var subtitleCount int
	if err = db.QueryRow(`SELECT count(*) FROM class_video_subtitles WHERE video_id=$1`, videoID).Scan(&subtitleCount); err != nil || subtitleCount != 0 {
		t.Fatal("subtitle delete", subtitleCount, err)
	}
	if _, err = db.Exec(`UPDATE class_videos SET viewer_ids='[2,3]'::jsonb WHERE id=$1`, videoID); err != nil {
		t.Fatal(err)
	}
	var audience string
	var viewerIDs string
	if err = db.QueryRow(`SELECT audience,viewer_ids::text FROM class_videos WHERE id=$1`, videoID).Scan(&audience, &viewerIDs); err != nil {
		t.Fatal(err)
	}
	if audience != "selected" || viewerIDs != "[2, 3]" {
		t.Fatal("title edit changed audience", audience, viewerIDs)
	}
	assertStatus(call("PUT", fmt.Sprintf("videos/%d", videoID), `{"title":"Bad edit","url":"https://youtu.be/dQw4w9WgXcQ","audience":"all","viewerIds":[]}`, "2"), 403)
	assertStatus(call("PUT", fmt.Sprintf("videos/%d/audience", videoID), `{"audience":"all","viewerIds":[]}`, "2"), 403)
	assertStatus(call("PUT", fmt.Sprintf("videos/%d/audience", videoID), `{"audience":"all","viewerIds":[]}`, "1"), 204)
	// An explicit role from one class cannot be referenced by a member in another class.
	r := httptest.NewRequest("PUT", fmt.Sprintf("/v1/classes/%d/access/members/3", other.ID), strings.NewReader(fmt.Sprintf(`{"roleKeys":[%q]}`, role.Key)))
	r.Header.Set("X-User-ID", "99")
	w = httptest.NewRecorder()
	h.classRouter(w, r)
	assertStatus(w, 422)
	data := []byte("%PDF-1.7\nprivate homework")
	r = assignmentRequest(t, "POST", assignmentInput{Title: "Bài tập thật", Instructions: "Làm câu 1", Status: "draft"}, "Đề.pdf", data)
	r.URL.Path = fmt.Sprintf("/v1/classes/%d/assignments", c.ID)
	w = httptest.NewRecorder()
	h.classRouter(w, r)
	assertStatus(w, 201)
	var assignment classAssignment
	if err = json.Unmarshal(w.Body.Bytes(), &assignment); err != nil {
		t.Fatal(err)
	}
	if len(assignment.Files) != 1 {
		t.Fatal(assignment)
	}
	w = call("GET", "assignments", "", "3")
	assertStatus(w, 200)
	if strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatal("draft exposed", w.Body.String())
	}
	filePath := fmt.Sprintf("assignments/%d/files/%d", assignment.ID, assignment.Files[0].ID)
	assertStatus(call("GET", filePath, "", "3"), 404)
	// A failed attachment deletion rolls back metadata and status changes too.
	r = assignmentRequest(t, "PUT", assignmentInput{Title: "Should roll back", Status: "published", DeleteFileIDs: []int64{999999}}, "", nil)
	r.URL.Path = fmt.Sprintf("/v1/classes/%d/assignments/%d", c.ID, assignment.ID)
	w = httptest.NewRecorder()
	h.classRouter(w, r)
	assertStatus(w, 422)
	var title, status string
	if err = db.QueryRow(`SELECT title,status FROM class_assignments WHERE id=$1`, assignment.ID).Scan(&title, &status); err != nil {
		t.Fatal(err)
	}
	if title != "Bài tập thật" || status != "draft" {
		t.Fatal("partial transaction committed", title, status)
	}
	r = assignmentRequest(t, "PUT", assignmentInput{Title: "Bài tập thật", Instructions: "Làm câu 1", Status: "published"}, "", nil)
	r.URL.Path = fmt.Sprintf("/v1/classes/%d/assignments/%d", c.ID, assignment.ID)
	r.Header.Set("X-User-ID", "2")
	w = httptest.NewRecorder()
	h.classRouter(w, r)
	assertStatus(w, 200)
	w = call("GET", filePath, "", "3")
	assertStatus(w, 200)
	if !bytes.Equal(w.Body.Bytes(), data) {
		t.Fatal("attachment corruption")
	}
	r = httptest.NewRequest("GET", fmt.Sprintf("/v1/classes/%d/%s", other.ID, filePath), nil)
	r.Header.Set("X-User-ID", "3")
	w = httptest.NewRecorder()
	h.classRouter(w, r)
	assertStatus(w, 404)
	assertStatus(call("DELETE", fmt.Sprintf("assignments/%d", assignment.ID), "", "1"), 204)
	var files int
	if err = db.QueryRow(`SELECT count(*) FROM class_assignment_files WHERE assignment_id=$1`, assignment.ID).Scan(&files); err != nil || files != 0 {
		t.Fatal("file cleanup", files, err)
	}
	assertStatus(call("DELETE", "members/2", "", "1"), 200)
	var grants int
	if err = db.QueryRow(`SELECT count(*) FROM class_member_access WHERE class_id=$1 AND user_id=2`, c.ID).Scan(&grants); err != nil || grants != 0 {
		t.Fatal("stale assignment after removal", grants, err)
	}
	assertStatus(call("DELETE", "access/roles/"+role.Key, "", "1"), 204)
}

func mustJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(raw)
}
