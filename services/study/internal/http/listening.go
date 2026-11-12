package http

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/subtitles"
)

type transcriptionClient struct {
	base, token string
	http        *http.Client
}

var transcriptionID = regexp.MustCompile(`^[a-f0-9]{32}$`)

type transcriptionView struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Error   string `json:"error,omitempty"`
	SRTText string `json:"srtText,omitempty"`
}

func (h *Handler) WithSTT(base, token string) *Handler {
	if base != "" && token != "" {
		h.stt = &transcriptionClient{strings.TrimRight(base, "/"), token, &http.Client{Timeout: 2 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	}
	return h
}
func (c *transcriptionClient) request(ctx context.Context, method, path string, body io.Reader, size int64) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.ContentLength = size
		req.Header.Set("Content-Type", "application/octet-stream")
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	return c.http.Do(req)
}
func (h *Handler) transcription(ctx context.Context, id string) (transcriptionView, error) {
	var view transcriptionView
	response, err := h.stt.request(ctx, "GET", "/v1/transcriptions/"+id, nil, 0)
	if err != nil {
		return view, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return view, fmt.Errorf("STT status %d", response.StatusCode)
	}
	if err = json.NewDecoder(io.LimitReader(response.Body, 16384)).Decode(&view); err != nil {
		return view, err
	}
	if view.ID != id {
		return view, errors.New("STT job mismatch")
	}
	if view.Status == "completed" {
		result, err := h.stt.request(ctx, "GET", "/v1/transcriptions/"+id+"/srt", nil, 0)
		if err != nil {
			return view, err
		}
		defer result.Body.Close()
		if result.StatusCode != 200 {
			return view, errors.New("STT subtitle unavailable")
		}
		data, err := io.ReadAll(io.LimitReader(result.Body, subtitles.MaxBytes+1))
		if err != nil {
			return view, err
		}
		view.SRTText, err = subtitles.Normalize(string(data))
		if err != nil || view.SRTText == "" {
			return view, errors.New("invalid STT subtitle")
		}
	}
	return view, nil
}
func (h *Handler) listeningRouter(w http.ResponseWriter, r *http.Request, setID int64, parts []string) {
	protectedHeaders(w)
	uid := userIDFromHeader(r)
	if err := h.quizQuestions.RequireOwner(r.Context(), setID, uid); err != nil {
		WriteServiceError(w, err)
		return
	}
	if len(parts) == 1 && parts[0] == "audio" && r.Method == "GET" {
		source := r.URL.Query().Get("source")
		if !strings.HasPrefix(source, "private-audio:") || h.quizAudio == nil {
			WriteError(w, 422, "Chọn file nghe đã tải lên kho riêng.")
			return
		}
		reader, mime, err := h.quizAudio.Open(r.Context(), setID, source)
		if err != nil {
			WriteError(w, 404, "Không tìm thấy file nghe trong học phần.")
			return
		}
		defer reader.Close()
		w.Header().Set("Content-Type", mime)
		http.ServeContent(w, r, "audio", time.Time{}, reader)
		return
	}
	if len(parts) == 0 || parts[0] != "transcriptions" || len(parts) > 2 {
		WriteError(w, 404, "Không tìm thấy chức năng bài nghe.")
		return
	}
	if h.stt == nil {
		WriteError(w, 503, "Chưa cấu hình kết nối tới máy tạo phụ đề STT.")
		return
	}
	if len(parts) == 1 && r.Method == "POST" {
		var in struct {
			AudioURL string `json:"audioUrl"`
			Language string `json:"language"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&in) != nil || !strings.HasPrefix(in.AudioURL, "private-audio:") {
			WriteError(w, 400, "Tải file nghe lên kho riêng trước khi tạo phụ đề.")
			return
		}
		if in.Language == "" {
			in.Language = "auto"
		}
		if in.Language != "auto" && in.Language != "vi" && in.Language != "ja" && in.Language != "en" {
			WriteError(w, 400, "Chọn ngôn ngữ vi, ja, en hoặc auto.")
			return
		}
		if h.quizAudio == nil || h.protectedQuiz == nil {
			WriteError(w, 503, "Kho file nghe chưa sẵn sàng.")
			return
		}
		if err := h.protectedQuiz.Allow(r.Context(), uid, "transcribe", 10, time.Hour); err != nil {
			h.playError(w, r, err)
			return
		}
		reader, _, err := h.quizAudio.Open(r.Context(), setID, in.AudioURL)
		if err != nil {
			WriteError(w, 404, "Không tìm thấy file nghe trong học phần.")
			return
		}
		defer reader.Close()
		size, err := reader.Seek(0, io.SeekEnd)
		if err != nil {
			WriteError(w, 500, "Không đọc được file nghe.")
			return
		}
		if _, err = reader.Seek(0, io.SeekStart); err != nil {
			WriteError(w, 500, "Không đọc được file nghe.")
			return
		}
		response, err := h.stt.request(r.Context(), "POST", "/v1/transcriptions?language="+url.QueryEscape(in.Language), reader, size)
		if err != nil {
			WriteError(w, 502, "Không kết nối được STT. Vui lòng thử lại.")
			return
		}
		defer response.Body.Close()
		if response.StatusCode == 429 {
			WriteError(w, 429, "Hàng đợi tạo phụ đề đang đầy. Vui lòng thử lại sau.")
			return
		}
		var view transcriptionView
		if response.StatusCode != 202 || json.NewDecoder(io.LimitReader(response.Body, 16384)).Decode(&view) != nil || !transcriptionID.MatchString(view.ID) {
			WriteError(w, 502, "STT không nhận được tác vụ. Vui lòng thử lại.")
			return
		}
		if _, err = h.db.ExecContext(r.Context(), `INSERT INTO quiz_transcription_jobs(job_id,study_set_id,user_id,audio_ref) VALUES($1,$2,$3,$4)`, view.ID, setID, uid, in.AudioURL); err != nil {
			WriteError(w, 500, "Không lưu được tác vụ tạo phụ đề.")
			return
		}
		WriteJSON(w, 202, view)
		return
	}
	if r.Method == "GET" {
		var id string
		if len(parts) == 2 {
			if !transcriptionID.MatchString(parts[1]) {
				WriteError(w, 404, "Không tìm thấy tác vụ.")
				return
			}
			err := h.db.QueryRowContext(r.Context(), `SELECT job_id FROM quiz_transcription_jobs WHERE job_id=$1 AND study_set_id=$2 AND user_id=$3`, parts[1], setID, uid).Scan(&id)
			if err != nil {
				WriteError(w, 404, "Không tìm thấy tác vụ của bạn trong học phần.")
				return
			}
		} else {
			err := h.db.QueryRowContext(r.Context(), `SELECT job_id FROM quiz_transcription_jobs WHERE study_set_id=$1 AND user_id=$2 AND audio_ref=$3 ORDER BY created_at DESC LIMIT 1`, setID, uid, r.URL.Query().Get("audioUrl")).Scan(&id)
			if errors.Is(err, sql.ErrNoRows) {
				WriteJSON(w, 200, map[string]any{"job": nil})
				return
			}
			if err != nil {
				WriteServiceError(w, err)
				return
			}
		}
		view, err := h.transcription(r.Context(), id)
		if err != nil {
			WriteError(w, 502, "Chưa lấy được trạng thái STT. Nếu tác vụ đã quá 24 giờ, hãy tạo lại.")
			return
		}
		WriteJSON(w, 200, map[string]any{"job": view})
		return
	}
	WriteError(w, 405, "Phương thức không được hỗ trợ.")
}
