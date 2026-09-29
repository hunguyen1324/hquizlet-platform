package http

import (
	"encoding/json"
	"errors"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/repository"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/storage"
	"log"
	"net/http"
	"strconv"
	"time"
)

func protectedHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
}
func (h *Handler) playError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, repository.ErrQuizRateLimit) || errors.Is(err, repository.ErrQuizSessionLimit) {
		log.Printf("[quiz-security] limit user=%d", userIDFromHeader(r))
		w.Header().Set("Retry-After", "60")
		WriteError(w, 429, "Quá nhiều yêu cầu hoặc đã có 2 phiên làm bài. Hãy nộp phiên cũ hoặc thử lại sau.")
		return
	}
	WriteServiceError(w, err)
}
func (h *Handler) protectedQuizRouter(w http.ResponseWriter, r *http.Request, setID int64, parts []string) {
	protectedHeaders(w)
	uid := userIDFromHeader(r)
	if h.protectedQuiz == nil {
		WriteError(w, 503, "quiz service unavailable")
		return
	}
	if len(parts) == 0 {
		if r.Method == "GET" {
			view, err := h.protectedQuiz.Summary(r.Context(), setID, uid)
			if err != nil {
				h.playError(w, r, err)
				return
			}
			WriteJSON(w, 200, view)
			return
		}
		if r.Method == "POST" {
			var in model.StartQuizInput
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&in); err != nil {
				WriteError(w, 400, "invalid request")
				return
			}
			view, err := h.protectedQuiz.Start(r.Context(), setID, uid, in)
			if err != nil {
				h.playError(w, r, err)
				return
			}
			WriteJSON(w, 201, view)
			return
		}
		WriteError(w, 405, "method not allowed")
		return
	}
	// Uploads require ownership and use no client-selected filesystem/object path.
	if len(parts) == 1 && parts[0] == "audio" && r.Method == "POST" {
		if err := h.quizQuestions.RequireOwner(r.Context(), setID, uid); err != nil {
			h.playError(w, r, err)
			return
		}
		if err := h.protectedQuiz.Allow(r.Context(), uid, "upload", 20, time.Hour); err != nil {
			h.playError(w, r, err)
			return
		}
		if h.quizAudio == nil {
			WriteError(w, 503, "audio storage unavailable")
			return
		}
		ref, err := h.quizAudio.Put(r.Context(), setID, http.MaxBytesReader(w, r.Body, storage.MaxQuizAudioBytes+1))
		if err != nil {
			WriteError(w, 422, "File nghe không hợp lệ hoặc lớn hơn 30 MB.")
			return
		}
		WriteJSON(w, 201, map[string]string{"audioUrl": ref})
		return
	}
	if len(parts) > 2 || len(parts[0]) != 48 {
		WriteError(w, 404, "session not found")
		return
	}
	id := parts[0]
	action := "page"
	if len(parts) == 2 {
		action = parts[1]
	}
	index := -1
	if raw := r.URL.Query().Get("index"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			WriteError(w, 400, "invalid question index")
			return
		}
		index = n
	}
	if (action == "audio" || action == "audio-ticket") && r.Method == "GET" {
		source, err := h.protectedQuiz.Audio(r.Context(), setID, uid, id, index)
		if err != nil {
			h.playError(w, r, err)
			return
		}
		if h.quizAudio == nil {
			WriteError(w, 503, "audio storage unavailable")
			return
		}
		reader, mime, err := h.quizAudio.Open(r.Context(), setID, source)
		if err != nil {
			WriteError(w, 422, "Không tải được file nghe. Chủ bài cần tải file lên kho riêng hoặc cấu hình nguồn audio được phép.")
			return
		}
		if action == "audio-ticket" {
			ticket := signAudioTicket(audioTicket{SetID: setID, UserID: uid, Session: id, Index: index, Expires: time.Now().Add(2 * time.Hour).Unix()})
			WriteJSON(w, 200, map[string]string{"path": "/v1/quiz-audio?ticket=" + ticket})
			return
		}
		w.Header().Set("Content-Type", mime)
		w.Header().Set("Content-Disposition", "inline")
		http.ServeContent(w, r, "audio", time.Time{}, reader)
		return
	}
	if (action == "page" && r.Method != "GET") || (action != "page" && r.Method != "POST") {
		WriteError(w, 405, "method not allowed")
		return
	}
	answer := ""
	if action == "answer" {
		var in struct {
			Index  int    `json:"index"`
			Answer string `json:"answer"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&in); err != nil {
			WriteError(w, 400, "invalid answer")
			return
		}
		index = in.Index
		answer = in.Answer
	}
	view, err := h.protectedQuiz.Act(r.Context(), setID, uid, id, action, index, answer)
	if err != nil {
		h.playError(w, r, err)
		return
	}
	WriteJSON(w, 200, view)
}
