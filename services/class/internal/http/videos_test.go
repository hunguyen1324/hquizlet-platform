package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/repository"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/service"
)

func TestYouTubeID(t *testing.T) {
	for _, raw := range []string{"https://youtu.be/dQw4w9WgXcQ?si=abc", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "https://m.youtube.com/shorts/dQw4w9WgXcQ", "https://youtube.com/embed/dQw4w9WgXcQ"} {
		if got := youtubeID(raw); got != "dQw4w9WgXcQ" {
			t.Errorf("%s: %q", raw, got)
		}
	}
	for _, raw := range []string{"https://youtube.com.evil.test/watch?v=dQw4w9WgXcQ", "http://youtube.com/watch?v=dQw4w9WgXcQ", "https://youtube.com/watch?v=bad", "https://evil.test/embed/dQw4w9WgXcQ", "https://youtube.com:444/watch?v=dQw4w9WgXcQ"} {
		if got := youtubeID(raw); got != "" {
			t.Errorf("accepted %s", raw)
		}
	}
}

type videoClassStore struct{ repository.ClassStore }

func (videoClassStore) GetByID(context.Context, int64) (*model.Class, error) {
	return &model.Class{OwnerUserID: 1}, nil
}

type videoMemberStore struct {
	repository.MemberStore
	role string
	err  error
}

func (s videoMemberStore) GetRole(context.Context, int64, int64) (string, error) {
	return s.role, s.err
}

func TestVideoAuthorizationFailsBeforeDatabase(t *testing.T) {
	for _, tc := range []struct {
		name, uid, role, method string
		err                     error
		status                  int
	}{
		{"anonymous", "", "", http.MethodGet, nil, 401},
		{"outsider", "2", "", http.MethodGet, nil, 403},
		{"student cannot create", "2", "student", http.MethodPost, nil, 403},
		{"student cannot change permissions", "2", "student", http.MethodPut, nil, 403},
		{"student cannot delete", "2", "student", http.MethodDelete, nil, 403},
		{"membership lookup fails closed", "2", "", http.MethodGet, errors.New("database unavailable"), 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			classes := videoClassStore{}
			members := videoMemberStore{role: tc.role, err: tc.err}
			h := &Handler{classes: service.NewClassService(classes, members), members: service.NewMemberService(classes, members)}
			r := httptest.NewRequest(tc.method, "/v1/classes/1/videos", nil)
			r.Header.Set("X-User-ID", tc.uid)
			parts := []string{}
			if tc.method == http.MethodPut || tc.method == http.MethodDelete {
				parts = []string{"1"}
			}
			w := httptest.NewRecorder()
			h.videoRouter(w, r, 1, parts)
			if w.Code != tc.status {
				t.Fatalf("got %d, want %d", w.Code, tc.status)
			}
		})
	}
}
