package http

import (
	"context"
	"encoding/json"
	"github.com/DATA-DOG/go-sqlmock"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFFmpegProgressHandlesSplitRecordsAndCapsBeforeReady(t *testing.T) {
	var values []int
	w := &ffmpegProgressWriter{duration: 100, highest: 5, report: func(p int, stage string) {
		if stage != "encoding" {
			t.Fatal(stage)
		}
		values = append(values, p)
	}}
	for _, data := range []string{"out_time_", "us=50000000\nprogress=continue\nout_time_us=25000000\n", "out_time_us=bad\nout_time_us=-1\nout_time_us=200000000\nprogress=end\n"} {
		if _, e := w.Write([]byte(data)); e != nil {
			t.Fatal(e)
		}
	}
	if len(values) != 2 || values[0] != 47 || values[1] != 90 {
		t.Fatal(values)
	}
	unknown := &ffmpegProgressWriter{report: func(int, string) { t.Fatal("invented percentage without duration") }}
	unknown.Write([]byte("out_time_us=2000000\nprogress=end\n"))
}

func TestProgressReporterThrottlesButReportsStageChanges(t *testing.T) {
	h, _, _ := privateTestHandler(t, "owner")
	var got []string
	h.videoProgress = func(_ context.Context, id int64, p int, stage string) {
		if id != 3 {
			t.Fatal(id)
		}
		got = append(got, stage)
	}
	report := h.videoProgressReporter(context.Background(), 3)
	report(0, "preparing")
	report(1, "preparing")
	report(5, "encoding")
	report(6, "encoding")
	report(90, "storing")
	if strings.Join(got, ",") != "preparing,encoding,storing" {
		t.Fatal(got)
	}
}

func TestDatabaseVideoProgressCanPersistUnknownDuration(t *testing.T) {
	h, mock, _ := privateTestHandler(t, "owner")
	mock.ExpectExec("INSERT INTO class_video_processing_progress").WithArgs(int64(3), nil, "encoding").WillReturnResult(sqlmock.NewResult(0, 1))
	databaseVideoProgress(h.db)(context.Background(), 3, -1, "encoding")
	if e := mock.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}

func TestVideoListIncludesProgressWithoutExposingOtherViewers(t *testing.T) {
	h, mock, _ := privateTestHandler(t, "student")
	mock.ExpectQuery("SELECT v.id,v.title.*LEFT JOIN class_video_processing_progress").WithArgs(int64(1), false, "[2]").WillReturnRows(sqlmock.NewRows([]string{"id", "title", "youtube_id", "audience", "viewer_ids", "source", "size_bytes", "processing_state", "percent", "stage"}).AddRow(3, "Lesson", "", "selected", []byte("[2,4]"), "sftp", 100, "processing", 47, "encoding").AddRow(4, "Unknown duration", "", "all", []byte("[]"), "sftp", 100, "processing", nil, "encoding").AddRow(5, "Ready", "", "all", []byte("[]"), "sftp", 100, "ready", 100, "ready"))
	r := httptest.NewRequest("GET", "/v1/classes/1/videos", nil)
	r.Header.Set("X-User-ID", "2")
	w := httptest.NewRecorder()
	h.videoRouter(w, r, 1, nil)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var videos []classVideo
	if e := json.Unmarshal(w.Body.Bytes(), &videos); e != nil {
		t.Fatal(e)
	}
	if len(videos) != 3 || videos[0].ProcessingPercent == nil || *videos[0].ProcessingPercent != 47 || videos[0].ProcessingStage != "encoding" || len(videos[0].ViewerIDs) != 0 {
		t.Fatal(w.Body.String())
	}
	if videos[1].ProcessingPercent != nil || *videos[2].ProcessingPercent != 100 {
		t.Fatal(w.Body.String())
	}
	if e := mock.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}

func TestActualFFmpegReportsEncodingAndStorage(t *testing.T) {
	h, _, _ := privateTestHandler(t, "owner")
	h.videoMaxBytes = 1024 * 1024
	seenEncoding, seenStorage, measuredEncoding := false, false, false
	err := h.putHLSWithProgress(context.Background(), strings.Repeat("b", 32)+".mp4", strings.NewReader(string(realVideoFixture(t))), func(p int, stage string) {
		if p >= 100 {
			t.Error("100% before ready commit")
		}
		if stage == "encoding" {
			seenEncoding = true
			if p > 5 {
				measuredEncoding = true
			}
		}
		if stage == "storing" {
			seenStorage = true
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !seenEncoding || !seenStorage || !measuredEncoding {
		t.Fatal("missing measured progress", seenEncoding, seenStorage, measuredEncoding)
	}
}
