package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/repository"
)

// quizSessionsRouter handles:
//   GET    /v1/quiz-sessions                         — list all sessions for the caller
//   GET    /v1/quiz-sessions?study_set_id=<n>        — filter by quiz
//   DELETE /v1/quiz-sessions/<id>                    — soft-delete one session
func (h *Handler) quizSessionsRouter(w http.ResponseWriter, r *http.Request) {
	protectedHeaders(w)
	uid := userIDFromHeader(r)
	if h.protectedQuiz == nil {
		WriteError(w, 503, "quiz service unavailable")
		return
	}

	// Strip prefix "/v1/quiz-sessions/" to get the optional session ID.
	parts := PathParts(r.URL.Path, "/v1/quiz-sessions/")

	// ── DELETE /v1/quiz-sessions/<id> ──────────────────────────────────────
	if len(parts) == 1 {
		id := parts[0]
		if len(id) != 48 {
			WriteError(w, 400, "invalid session id")
			return
		}
		if r.Method != http.MethodDelete {
			WriteError(w, 405, "method not allowed")
			return
		}
		if err := h.protectedQuiz.DeleteSession(r.Context(), id, uid); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				// Idempotent: already deleted counts as success.
				WriteJSON(w, 200, map[string]bool{"ok": true})
				return
			}
			WriteServiceError(w, err)
			return
		}
		WriteJSON(w, 200, map[string]bool{"ok": true})
		return
	}

	// ── GET /v1/quiz-sessions ──────────────────────────────────────────────
	if len(parts) != 0 {
		WriteError(w, 404, "not found")
		return
	}
	if r.Method != http.MethodGet {
		WriteError(w, 405, "method not allowed")
		return
	}
	q := r.URL.Query()
	var studySetID int64
	if raw := strings.TrimSpace(q.Get("study_set_id")); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n <= 0 {
			WriteError(w, 400, "invalid study_set_id")
			return
		}
		studySetID = n
	}
	page := intQueryParam(q.Get("page"), 1)
	perPage := intQueryParam(q.Get("per_page"), 20)

	metas, total, err := h.protectedQuiz.ListSessions(r.Context(), uid, studySetID, page, perPage)
	if err != nil {
		WriteServiceError(w, err)
		return
	}
	WriteJSON(w, 200, map[string]any{
		"items":   metas,
		"total":   total,
		"page":    page,
		"perPage": perPage,
	})
}

// quizSessionsDeleteRouter handles DELETE /v1/quiz-sessions/<id> when the
// mux is registered without a trailing slash (exact match handler).
func (h *Handler) quizSessionDeleteExact(w http.ResponseWriter, r *http.Request) {
	h.quizSessionsRouter(w, r)
}

// writeQuizSessionLimitError emits the structured 409 response for the
// per-quiz session cap, including quota context for the frontend.
func writeQuizSessionLimitError(w http.ResponseWriter, studySetID int64, activeCount, limit int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusConflict)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code":        "quiz_session_limit",
		"message":     "Đã đủ 5 phiên đang làm cho quiz này. Tiếp tục hoặc xóa một phiên để tạo phiên mới.",
		"studySetId":  studySetID,
		"activeCount": activeCount,
		"limit":       limit,
		"details":     map[string]any{},
	})
}
