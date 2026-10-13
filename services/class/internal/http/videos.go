package http

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	classservice "github.com/hunguyen1324/hquizlet-platform/services/class/internal/service"
)

type classVideo struct {
	ID              int64   `json:"id"`
	Title           string  `json:"title"`
	YouTubeID       string  `json:"youtubeId"`
	Audience        string  `json:"audience"`
	ViewerIDs       []int64 `json:"viewerIds"`
	Source          string  `json:"source,omitempty"`
	SizeBytes       int64   `json:"sizeBytes,omitempty"`
	ProcessingState string  `json:"processingState,omitempty"`
}

func youtubeID(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return ""
	}
	var id string
	switch strings.ToLower(u.Hostname()) {
	case "youtu.be":
		id = strings.TrimPrefix(u.Path, "/")
	case "youtube.com", "www.youtube.com", "m.youtube.com":
		if u.Path == "/watch" {
			id = u.Query().Get("v")
		} else {
			p := strings.Split(strings.Trim(u.Path, "/"), "/")
			if len(p) == 2 && (p[0] == "embed" || p[0] == "shorts" || p[0] == "live") {
				id = p[1]
			}
		}
	}
	if len(id) != 11 || !regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(id) {
		return ""
	}
	return id
}

func (h *Handler) videoRouter(w http.ResponseWriter, r *http.Request, classID int64, parts []string) {
	w.Header().Set("Cache-Control", "private, no-store")
	uid := userIDFromHeader(r)
	cls, err := h.classes.GetByID(r.Context(), classID, uid)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	role, err := h.members.GetRole(r.Context(), classID, uid)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	staff := cls.OwnerUserID == uid || role == "teacher" || role == "owner"
	if !staff && role == "" {
		writeServiceError(w, r, classservice.ErrForbidden)
		return
	}
	if len(parts) == 1 && parts[0] == "upload" && r.Method == http.MethodPost {
		if !staff {
			writeServiceError(w, r, classservice.ErrForbidden)
			return
		}
		h.uploadPrivateVideo(w, r, classID, cls.OwnerUserID)
		return
	}
	if len(parts) == 2 && parts[1] == "playback" && r.Method == http.MethodPost {
		id, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil || id <= 0 {
			WriteError(w, 400, "invalid video id")
			return
		}
		h.privateVideoPlayback(w, r, classID, id, uid, staff)
		return
	}
	if len(parts) > 1 {
		WriteError(w, 404, "not found")
		return
	}
	var id int64
	if len(parts) == 1 {
		id, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil || id <= 0 {
			WriteError(w, 400, "invalid video id")
			return
		}
	}
	if r.Method == http.MethodGet && id == 0 {
		viewer, _ := json.Marshal([]int64{uid})
		rows, err := h.db.QueryContext(r.Context(), `SELECT id,title,COALESCE(youtube_id,''),audience,viewer_ids,CASE WHEN storage_key IS NULL THEN 'youtube' ELSE 'sftp' END,COALESCE(size_bytes,0),processing_state FROM class_videos WHERE class_id=$1 AND ($2 OR audience='all' OR viewer_ids @> $3::jsonb) ORDER BY created_at DESC,id DESC`, classID, staff, string(viewer))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		defer rows.Close()
		videos := []classVideo{}
		for rows.Next() {
			v := classVideo{}
			var raw []byte
			if err := rows.Scan(&v.ID, &v.Title, &v.YouTubeID, &v.Audience, &raw, &v.Source, &v.SizeBytes, &v.ProcessingState); err != nil {
				writeServiceError(w, r, err)
				return
			}
			if err := json.Unmarshal(raw, &v.ViewerIDs); err != nil {
				writeServiceError(w, r, err)
				return
			}
			if !staff {
				v.ViewerIDs = []int64{}
			}
			videos = append(videos, v)
		}
		if err := rows.Err(); err != nil {
			writeServiceError(w, r, err)
			return
		}
		WriteJSON(w, 200, videos)
		return
	}
	if !staff {
		writeServiceError(w, r, classservice.ErrForbidden)
		return
	}
	if r.Method == http.MethodDelete && id > 0 {
		// Remove stored videos through the private storage path, not the public file service.
		var key sql.NullString
		err := h.db.QueryRowContext(r.Context(), `SELECT storage_key FROM class_videos WHERE class_id=$1 AND id=$2`, classID, id).Scan(&key)
		if err == sql.ErrNoRows {
			WriteError(w, 404, "video not found")
			return
		}
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		if key.Valid {
			if !h.privateVideosEnabled(w) {
				return
			}
			if err := h.videoStore.Delete(r.Context(), key.String); err != nil {
				WriteError(w, 502, "Không xóa được video trên kho lưu trữ")
				return
			}
		}
		result, err := h.db.ExecContext(r.Context(), `DELETE FROM class_videos WHERE class_id=$1 AND id=$2`, classID, id)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		n, err := result.RowsAffected()
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		if n == 0 {
			WriteError(w, 404, "video not found")
			return
		}
		WriteJSON(w, 200, map[string]bool{"ok": true})
		return
	}
	if (r.Method == http.MethodPost && id == 0) || (r.Method == http.MethodPut && id > 0) {
		var in struct {
			Title     string  `json:"title"`
			URL       string  `json:"url"`
			Audience  string  `json:"audience"`
			ViewerIDs []int64 `json:"viewerIds"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32768)).Decode(&in); err != nil {
			WriteError(w, 400, "invalid JSON body")
			return
		}
		in.Title = strings.TrimSpace(in.Title)
		if r.Method == http.MethodPut {
			var stored bool
			if err := h.db.QueryRowContext(r.Context(), `SELECT storage_key IS NOT NULL FROM class_videos WHERE class_id=$1 AND id=$2`, classID, id).Scan(&stored); err != nil {
				if err == sql.ErrNoRows {
					WriteError(w, 404, "video not found")
				} else {
					writeServiceError(w, r, err)
				}
				return
			}
			if stored {
				h.updatePrivateVideo(w, r, classID, id, cls.OwnerUserID, in.Title, in.Audience, in.ViewerIDs)
				return
			}
		}
		yt := youtubeID(in.URL)
		if in.Title == "" || len([]rune(in.Title)) > 200 || yt == "" || (in.Audience != "all" && in.Audience != "selected") || len(in.ViewerIDs) > 1000 {
			WriteError(w, 422, "Tiêu đề, liên kết YouTube hoặc quyền xem không hợp lệ")
			return
		}
		ids := []int64{}
		seen := map[int64]bool{}
		if in.Audience == "selected" {
			for _, viewer := range in.ViewerIDs {
				role, err := h.members.GetRole(r.Context(), classID, viewer)
				if err != nil {
					writeServiceError(w, r, err)
					return
				}
				if viewer <= 0 || (role == "" && viewer != cls.OwnerUserID) {
					WriteError(w, 422, "Người xem phải là thành viên nhóm")
					return
				}
				if !seen[viewer] {
					ids = append(ids, viewer)
					seen[viewer] = true
				}
			}
		}
		raw, _ := json.Marshal(ids)
		if id == 0 {
			err = h.db.QueryRowContext(r.Context(), `INSERT INTO class_videos(class_id,title,youtube_id,audience,viewer_ids) VALUES($1,$2,$3,$4,$5::jsonb) RETURNING id`, classID, in.Title, yt, in.Audience, string(raw)).Scan(&id)
		} else {
			var result interface{ RowsAffected() (int64, error) }
			result, err = h.db.ExecContext(r.Context(), `UPDATE class_videos SET title=$1,youtube_id=$2,audience=$3,viewer_ids=$4::jsonb WHERE class_id=$5 AND id=$6`, in.Title, yt, in.Audience, string(raw), classID, id)
			if err == nil {
				n, e := result.RowsAffected()
				err = e
				if err == nil && n == 0 {
					WriteError(w, 404, "video not found")
					return
				}
			}
		}
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		WriteJSON(w, 200, classVideo{ID: id, Title: in.Title, YouTubeID: yt, Audience: in.Audience, ViewerIDs: ids, Source: "youtube"})
		return
	}
	WriteError(w, 405, "method not allowed")
}
