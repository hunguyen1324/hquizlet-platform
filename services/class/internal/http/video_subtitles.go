package http

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/youtubecaptions"
)

// Every transcript request is scoped to the video, its class and its current audience.
func (h *Handler) subtitleRouter(w http.ResponseWriter, r *http.Request, classID int64, parts []string) {
	w.Header().Set("Cache-Control", "private, no-store")
	if len(parts) != 2 || parts[1] != "subtitles" {
		WriteError(w, 404, "not found")
		return
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		WriteError(w, 400, "invalid video id")
		return
	}
	if !h.requireClassAction(w, r, classID, "class-content:ViewVideo") {
		return
	}
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
	staff := cls.OwnerUserID == uid || role == "owner" || role == "teacher"
	if h.access != nil {
		staff = h.hasClassAction(r, classID, "class-video:ManageAudience")
	}
	viewer, _ := json.Marshal([]int64{uid})
	var youtubeID string
	err = h.db.QueryRowContext(r.Context(), `SELECT youtube_id FROM class_videos WHERE class_id=$1 AND id=$2 AND youtube_id IS NOT NULL AND ($3 OR audience='all' OR viewer_ids @> $4::jsonb)`, classID, id, staff, string(viewer)).Scan(&youtubeID)
	if err == sql.ErrNoRows {
		WriteError(w, 404, "Không có video YouTube hoặc bạn không được xem video này")
		return
	}
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	if r.Method == http.MethodGet {
		language, translation := r.URL.Query().Get("language"), r.URL.Query().Get("translation")
		if !youtubecaptions.ValidLanguage(language) || !youtubecaptions.ValidLanguage(translation) {
			WriteError(w, 422, "Mã ngôn ngữ không hợp lệ")
			return
		}
		result := youtubecaptions.Result{AvailableTracks: []youtubecaptions.TrackInfo{}}
		rows, e := h.db.QueryContext(r.Context(), `SELECT kind,language,label,cues FROM class_video_subtitles WHERE video_id=$1 ORDER BY kind`, id)
		if e != nil {
			writeServiceError(w, r, e)
			return
		}
		for rows.Next() {
			var kind string
			var track youtubecaptions.Track
			var raw []byte
			if e = rows.Scan(&kind, &track.Language, &track.Label, &raw); e != nil {
				rows.Close()
				writeServiceError(w, r, e)
				return
			}
			if e = json.Unmarshal(raw, &track.Cues); e != nil {
				rows.Close()
				writeServiceError(w, r, e)
				return
			}
			track.Source = "uploaded"
			if kind == "original" {
				result.Original = &track
			} else {
				result.Translation = &track
			}
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			writeServiceError(w, r, e)
			return
		}
		if (result.Original == nil || language != "" || r.URL.Query().Get("youtube") == "1") && h.captions != nil {
			public, fetchErr := h.captions.Fetch(r.Context(), youtubeID, language, translation)
			result.AvailableTracks = public.AvailableTracks
			if fetchErr == nil {
				result.Original = public.Original
				if public.Translation != nil {
					result.Translation = public.Translation
				}
				result.Message = public.Message
			} else if result.Original == nil {
				result.Message = "YouTube chưa cung cấp phụ đề cho ứng dụng. Bạn vẫn có thể xem video hoặc nhập file SRT/VTT để học theo câu."
				if public.Message != "" {
					result.Message = public.Message
				}
			} else {
				result.Message = "Không lấy được phụ đề YouTube; đang dùng phụ đề đã lưu của lớp."
			}
		}
		if result.Original == nil && result.Message == "" {
			result.Message = "Chưa có phụ đề. Nhập SRT/VTT để học theo câu."
		}
		WriteJSON(w, 200, result)
		return
	}
	if h.access != nil {
		if !h.requireClassAction(w, r, classID, "class-video:Update") {
			return
		}
	} else if !staff {
		WriteError(w, 403, "Không có quyền sửa phụ đề")
		return
	}
	if r.Method == http.MethodPut {
		var in struct {
			Kind     string                `json:"kind"`
			Language string                `json:"language"`
			Label    string                `json:"label"`
			Cues     []youtubecaptions.Cue `json:"cues"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 2*1024*1024)).Decode(&in) != nil {
			WriteError(w, 422, "File phụ đề quá lớn hoặc dữ liệu không hợp lệ")
			return
		}
		in.Label = strings.TrimSpace(in.Label)
		if (in.Kind != "original" && in.Kind != "translation") || !youtubecaptions.ValidLanguage(in.Language) || in.Label == "" || len([]rune(in.Label)) > 100 {
			WriteError(w, 422, "Kiểm tra loại phụ đề, nhãn và ngôn ngữ")
			return
		}
		if err = youtubecaptions.ValidateCues(in.Cues); err != nil {
			WriteError(w, 422, err.Error())
			return
		}
		raw, _ := json.Marshal(in.Cues)
		_, err = h.db.ExecContext(r.Context(), `INSERT INTO class_video_subtitles(video_id,kind,language,label,cues,updated_by) VALUES($1,$2,$3,$4,$5::jsonb,$6) ON CONFLICT(video_id,kind) DO UPDATE SET language=EXCLUDED.language,label=EXCLUDED.label,cues=EXCLUDED.cues,updated_by=EXCLUDED.updated_by,updated_at=NOW()`, id, in.Kind, in.Language, in.Label, string(raw), uid)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		WriteJSON(w, 200, youtubecaptions.Track{Language: in.Language, Label: in.Label, Source: "uploaded", Cues: in.Cues})
		return
	}
	if r.Method == http.MethodDelete {
		kind := r.URL.Query().Get("kind")
		if kind != "original" && kind != "translation" {
			WriteError(w, 422, "Loại phụ đề không hợp lệ")
			return
		}
		if _, err = h.db.ExecContext(r.Context(), `DELETE FROM class_video_subtitles WHERE video_id=$1 AND kind=$2`, id, kind); err != nil {
			writeServiceError(w, r, err)
			return
		}
		w.WriteHeader(204)
		return
	}
	WriteError(w, 405, "method not allowed")
}
