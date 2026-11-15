package http

import (
	"encoding/json"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/youtubecaptions"
	"net/http/httptest"
	"strings"
	"testing"
)

func expectSubtitleVideo(mock sqlmock.Sqlmock, staff bool) {
	mock.ExpectQuery("SELECT youtube_id FROM class_videos WHERE class_id").WithArgs(int64(1), int64(7), staff, "[2]").WillReturnRows(sqlmock.NewRows([]string{"youtube_id"}).AddRow("M7lc1UVf-VE"))
}
func TestSubtitlesRespectCurrentVideoAudienceBeforeReadingCaptions(t *testing.T) {
	h, mock, _ := privateTestHandler(t, "student")
	mock.ExpectQuery("SELECT youtube_id FROM class_videos WHERE class_id").WithArgs(int64(1), int64(7), false, "[2]").WillReturnRows(sqlmock.NewRows([]string{"youtube_id"}))
	r := httptest.NewRequest("GET", "/v1/classes/1/videos/7/subtitles", nil)
	r.Header.Set("X-User-ID", "2")
	w := httptest.NewRecorder()
	h.subtitleRouter(w, r, 1, []string{"7", "subtitles"})
	if w.Code != 404 {
		t.Fatal(w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestSavedSubtitlesArePrivateAndStudentsCannotReplaceThem(t *testing.T) {
	h, mock, _ := privateTestHandler(t, "student")
	expectSubtitleVideo(mock, false)
	mock.ExpectQuery("SELECT kind,language,label,cues FROM class_video_subtitles").WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"kind", "language", "label", "cues"}).AddRow("original", "ja", "Japanese", `[{"start":0,"end":2,"text":"こんにちは"}]`))
	r := httptest.NewRequest("GET", "/v1/classes/1/videos/7/subtitles", nil)
	r.Header.Set("X-User-ID", "2")
	w := httptest.NewRecorder()
	h.subtitleRouter(w, r, 1, []string{"7", "subtitles"})
	var result youtubecaptions.Result
	if json.Unmarshal(w.Body.Bytes(), &result) != nil || result.Original == nil || result.Original.Cues[0].Text != "こんにちは" || w.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal(w.Code, w.Body.String())
	}
	expectSubtitleVideo(mock, false)
	r = httptest.NewRequest("PUT", "/v1/classes/1/videos/7/subtitles", strings.NewReader(`{"kind":"original","language":"ja","label":"Replacement","cues":[{"start":0,"end":2,"text":"new"}]}`))
	r.Header.Set("X-User-ID", "2")
	w = httptest.NewRecorder()
	h.subtitleRouter(w, r, 1, []string{"7", "subtitles"})
	if w.Code != 403 {
		t.Fatal(w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestSubtitleWriteValidatesCuesAndScopesStorage(t *testing.T) {
	h, mock, _ := privateTestHandler(t, "teacher")
	expectSubtitleVideo(mock, true)
	mock.ExpectExec("INSERT INTO class_video_subtitles").WithArgs(int64(7), "original", "ja", "Japanese", `[{"start":0,"end":2,"text":"こんにちは"}]`, int64(2)).WillReturnResult(sqlmock.NewResult(0, 1))
	r := httptest.NewRequest("PUT", "/v1/classes/1/videos/7/subtitles", strings.NewReader(`{"kind":"original","language":"ja","label":"Japanese","cues":[{"start":0,"end":2,"text":"<b>こんにちは</b>"}]}`))
	r.Header.Set("X-User-ID", "2")
	w := httptest.NewRecorder()
	h.subtitleRouter(w, r, 1, []string{"7", "subtitles"})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	expectSubtitleVideo(mock, true)
	r = httptest.NewRequest("PUT", "/v1/classes/1/videos/7/subtitles", strings.NewReader(`{"kind":"original","language":"ja","label":"Japanese","cues":[{"start":3,"end":2,"text":"bad"}]}`))
	r.Header.Set("X-User-ID", "2")
	w = httptest.NewRecorder()
	h.subtitleRouter(w, r, 1, []string{"7", "subtitles"})
	if w.Code != 422 {
		t.Fatal(w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestNoTranscriptDoesNotClaimYouTubeVideoHasNoCaptions(t *testing.T) {
	h, mock, _ := privateTestHandler(t, "student")
	expectSubtitleVideo(mock, false)
	mock.ExpectQuery("SELECT kind,language,label,cues FROM class_video_subtitles").WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"kind", "language", "label", "cues"}))
	r := httptest.NewRequest("GET", "/v1/classes/1/videos/7/subtitles", nil)
	r.Header.Set("X-User-ID", "2")
	w := httptest.NewRecorder()
	h.subtitleRouter(w, r, 1, []string{"7", "subtitles"})
	var result youtubecaptions.Result
	if json.Unmarshal(w.Body.Bytes(), &result) != nil || result.Original != nil || result.Message == "" || w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
