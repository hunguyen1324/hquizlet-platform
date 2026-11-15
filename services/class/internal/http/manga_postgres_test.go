package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"
)

func testMangaWorkspace(t *testing.T, h *Handler, classID int64) {
	t.Helper()
	h.mangaStore = &memoryVideoStore{}
	call := func(method, path, body, uid string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/v1/manga"+path, strings.NewReader(body))
		r.Header.Set("X-User-ID", uid)
		w := httptest.NewRecorder()
		h.mangaRouter(w, r)
		return w
	}
	status := func(w *httptest.ResponseRecorder, n int) {
		t.Helper()
		if w.Code != n {
			t.Fatalf("manga expected %d got %d %s", n, w.Code, w.Body.String())
		}
	}
	status(call("GET", "", "", ""), 401)
	created := call("POST", "", `{"sourceUuid":"manga-qa","title":"QA manga","volume":"1","pageCount":1}`, "1")
	status(created, 201)
	var result struct {
		ID int64 `json:"id"`
	}
	if json.Unmarshal(created.Body.Bytes(), &result) != nil || result.ID < 1 {
		t.Fatal(created.Body.String())
	}
	base := fmt.Sprintf("/%d", result.ID)
	status(call("GET", base, "", "2"), 404)
	status(call("POST", base+"/complete", "", "1"), 409)
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	form.WriteField("page", `{"img_width":16,"img_height":16,"blocks":[{"box":[-1,0,10,12],"vertical":true,"font_size":5,"lines":["OCR original"]}]}`)
	part, e := form.CreateFormFile("image", "1.png")
	if e != nil {
		t.Fatal(e)
	}
	if e = png.Encode(part, image.NewRGBA(image.Rect(0, 0, 16, 16))); e != nil {
		t.Fatal(e)
	}
	form.Close()
	payload := append([]byte{}, body.Bytes()...)
	upload := httptest.NewRequest("POST", "/v1/manga"+base+"/pages/0", &body)
	upload.Header.Set("X-User-ID", "1")
	upload.Header.Set("Content-Type", form.FormDataContentType())
	w := httptest.NewRecorder()
	h.mangaRouter(w, upload)
	status(w, 204)
	status(call("POST", base+"/complete", "", "1"), 204)
	status(call("GET", base+"/pages/0/image", "", "1"), 200)
	// Community imports become visible to other users once complete, but remain owner-editable.
	status(call("GET", base+"/pages/0", "", "2"), 200)
	status(call("PUT", base+"/pages/0", `{"revision":1,"lines":[["OCR fixed"]]}`, "2"), 403)
	status(call("PUT", base, `{"title":"QA manga","volume":"1","visibility":"shared"}`, "1"), 204)
	status(call("GET", base+"/pages/0", "", "2"), 200)
	status(call("PUT", base+"/pages/0", `{"revision":1,"lines":[["bad"]]}`, "2"), 403)
	w = call("PUT", base+"/pages/0", `{"revision":1,"lines":[["OCR fixed"]]}`, "1")
	status(w, 200)
	if !strings.Contains(w.Body.String(), "OCR fixed") {
		t.Fatal(w.Body.String())
	}
	source := `<html><head><title>QA | mokuro</title></head><body><div class="page"><div class="pageContainer" style='width:16; height:16; background-image:url("manga_images2/1.png");'><div class="textBox" style="left:0;top:0;width:10;height:12;font-size:5px"><p>OCR original</p></div></div></div><script>window.readerOriginal=true;</script></body></html>`
	inspected := call("POST", "/inspect", mustJSON(map[string]string{"htmlSource": source, "sourceName": "chapter.html"}), "1")
	status(inspected, 200)
	if !strings.Contains(inspected.Body.String(), "manga_images2/1.png") {
		t.Fatal(inspected.Body.String())
	}
	if _, e = h.db.Exec(`UPDATE manga_volumes SET html_source=$2,source_name='chapter.html' WHERE id=$1`, result.ID, source); e != nil {
		t.Fatal(e)
	}
	exported := call("GET", base+"/export.html", "", "1")
	status(exported, 200)
	if !strings.Contains(exported.Body.String(), "OCR fixed") || !strings.Contains(exported.Body.String(), "window.readerOriginal=true;") || !strings.Contains(exported.Header().Get("Content-Disposition"), "attachment") {
		t.Fatal("HTML export lost edit/reader", exported.Body.String())
	}
	status(call("PUT", base+"/pages/0", `{"revision":1,"lines":[["stale"]]}`, "1"), 409)
	w = call("PUT", base+"/pages/0", `{"revision":2,"restore":true}`, "1")
	status(w, 200)
	if !strings.Contains(w.Body.String(), "OCR original") {
		t.Fatal(w.Body.String())
	}
	status(call("PUT", base+"/pages/0", `{"revision":3,"lines":[]}`, "1"), 422)
	status(call("PUT", base, `{"title":"QA manga","volume":"1","visibility":"private"}`, "1"), 204)
	status(call("GET", base+"/pages/0/image", "", "2"), 404)
	status(call("DELETE", base, "", "1"), 204)
	status(call("GET", base, "", "1"), 404)
	var queued int
	if e = h.db.QueryRow(`SELECT count(*) FROM manga_image_gc`).Scan(&queued); e != nil || queued != 1 {
		t.Fatal("missing durable image cleanup", queued, e)
	}

	// The same reader and importer must keep classroom content out of the community library.
	scoped := call("POST", fmt.Sprintf("?classId=%d", classID), `{"sourceUuid":"manga-scoped-qa","title":"Group manga","volume":"1","pageCount":1}`, "1")
	status(scoped, 201)
	var group struct {
		ID int64 `json:"id"`
	}
	if json.Unmarshal(scoped.Body.Bytes(), &group) != nil || group.ID < 1 {
		t.Fatal(scoped.Body.String())
	}
	groupBase := fmt.Sprintf("/%d", group.ID)
	r := httptest.NewRequest("POST", "/v1/manga"+groupBase+"/pages/0", bytes.NewReader(payload))
	r.Header.Set("X-User-ID", "1")
	r.Header.Set("Content-Type", form.FormDataContentType())
	w = httptest.NewRecorder()
	h.mangaRouter(w, r)
	status(w, 204)
	status(call("POST", groupBase+"/complete", "", "1"), 204)
	status(call("GET", groupBase, "", "3"), 200)
	status(call("GET", groupBase, "", "99"), 403)
	community := call("GET", "", "", "1")
	status(community, 200)
	var publicVolumes []mangaVolume
	if json.Unmarshal(community.Body.Bytes(), &publicVolumes) != nil {
		t.Fatal(community.Body.String())
	}
	for _, v := range publicVolumes {
		if v.ID == group.ID {
			t.Fatal("classroom manga leaked into community library")
		}
	}
	groupList := call("GET", fmt.Sprintf("?classId=%d", classID), "", "3")
	status(groupList, 200)
	var groupVolumes []mangaVolume
	if json.Unmarshal(groupList.Body.Bytes(), &groupVolumes) != nil || len(groupVolumes) != 1 || groupVolumes[0].ID != group.ID || groupVolumes[0].Visibility != "shared" {
		t.Fatal("classroom default scope failed", groupList.Body.String())
	}
	status(call("DELETE", groupBase, "", "1"), 204)
}
