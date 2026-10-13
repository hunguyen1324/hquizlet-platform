package http

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestBackgroundVideoPublishAndFailure(t *testing.T) {
	for _, tc := range []struct {
		name       string
		fail       bool
		commitFail bool
	}{{"publishes encrypted output", false, false}, {"storage failure keeps source and marks failed", true, false}, {"database failure leaves durable job", false, true}} {
		t.Run(tc.name, func(t *testing.T) {
			h, mock, store := privateTestHandler(t, "owner")
			h.videoMaxBytes = 1024 * 1024
			store.body = realVideoFixture(t)
			source := strings.Repeat("a", 32) + ".mp4"
			output := strings.Repeat("a", 32) + ".webm"
			if tc.fail {
				store.putErr = errors.New("SFTP unavailable")
			}
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT id,storage_key,hls_output_key.*FOR UPDATE SKIP LOCKED").WillReturnRows(sqlmock.NewRows([]string{"id", "storage_key", "hls_output_key"}).AddRow(3, source, output))
			if tc.fail {
				mock.ExpectExec("UPDATE class_videos SET processing_state='failed'").WithArgs(int64(3)).WillReturnResult(sqlmock.NewResult(0, 1))
			} else {
				mock.ExpectExec("UPDATE class_videos SET storage_key").WithArgs(output, hlsContentType, int64(3)).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec("INSERT INTO class_video_deletions").WithArgs(source).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			if tc.commitFail {
				mock.ExpectCommit().WillReturnError(errors.New("database unavailable"))
			} else {
				mock.ExpectCommit()
			}
			found, err := h.processNextVideo(context.Background())
			if !found || (err != nil) != tc.commitFail {
				t.Fatalf("found=%v err=%v", found, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBackgroundVideoNoPendingJob(t *testing.T) {
	h, mock, _ := privateTestHandler(t, "owner")
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id,storage_key,hls_output_key").WillReturnRows(sqlmock.NewRows([]string{"id", "storage_key", "hls_output_key"}))
	mock.ExpectRollback()
	found, err := h.processNextVideo(context.Background())
	if found || err != nil {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
