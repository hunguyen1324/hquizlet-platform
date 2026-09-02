// Package http implements the HTTP handlers for live quiz endpoints.
// Dev 3 - [P6-GO-05, P6-SSE-01]
package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/quiz/internal/live/events"
	"github.com/hunguyen1324/hquizlet-platform/services/quiz/internal/live/model"
	"github.com/hunguyen1324/hquizlet-platform/services/quiz/internal/live/realtime"
	"github.com/hunguyen1324/hquizlet-platform/services/quiz/internal/live/service"
)

// Handlers exposes all live quiz HTTP endpoints.
type Handlers struct {
	svc         *service.Service
	broadcaster *realtime.Broadcaster
	scheduler   *Scheduler
}

// SetScheduler wires the question timer after handlers and scheduler are constructed.
func (h *Handlers) SetScheduler(scheduler *Scheduler) { h.scheduler = scheduler }

// NewHandlers creates a new handler set.
func NewHandlers(svc *service.Service, broadcaster *realtime.Broadcaster) *Handlers {
	return &Handlers{svc: svc, broadcaster: broadcaster}
}

// RegisterRoutes registers all live endpoints on the given mux.
func (h *Handlers) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/live-sessions", h.createSession)
	mux.HandleFunc("GET /v1/live-sessions/{sessionId}", h.getSession)
	mux.HandleFunc("POST /v1/live-sessions/{code}/join", h.joinSession)
	mux.HandleFunc("GET /v1/live-sessions/{sessionId}/player-state", h.playerState)
	mux.HandleFunc("GET /v1/live-sessions/{sessionId}/events", h.sseStream)
	mux.HandleFunc("POST /v1/live-sessions/{sessionId}/start", h.startSession)
	mux.HandleFunc("POST /v1/live-sessions/{sessionId}/questions/current/close", h.closeQuestion)
	mux.HandleFunc("POST /v1/live-sessions/{sessionId}/questions/next", h.nextQuestion)
	mux.HandleFunc("POST /v1/live-sessions/{sessionId}/answers", h.submitAnswer)
	mux.HandleFunc("GET /v1/live-sessions/{sessionId}/leaderboard", h.getLeaderboard)
	mux.HandleFunc("POST /v1/live-sessions/{sessionId}/end", h.endSession)
}

// extractHostUserID extracts the host user ID from the X-User-ID header (set by Gateway).
func extractHostUserID(r *http.Request) int64 {
	id, _ := strconv.ParseInt(r.Header.Get("X-User-ID"), 10, 64)
	return id
}

// extractParticipantID extracts the participant ID from the Authorization header.
// The participant token is in "Authorization: Bearer <token>" format.
func extractParticipantToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeJSON(w, status, map[string]interface{}{
		"code":      code,
		"message":   message,
		"requestId": r.Header.Get("X-Request-ID"),
		"details":   map[string]interface{}{},
	})
}

func parseSessionID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("sessionId"), 10, 64)
}

// createSession handles POST /v1/live-sessions
func (h *Handlers) createSession(w http.ResponseWriter, r *http.Request) {
	userID := extractHostUserID(r)
	if userID <= 0 {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	var req struct {
		StudySetID         int64 `json:"studySetId"`
		QuestionCount      int   `json:"questionCount"`
		QuestionDurationMs int   `json:"questionDurationMs"`
		Seed               int64 `json:"seed"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.QuestionCount < 1 || req.QuestionCount > 100 {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "questionCount must be 1-100")
		return
	}
	if req.QuestionDurationMs < 5000 || req.QuestionDurationMs > 120000 {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "questionDurationMs must be 5000-120000")
		return
	}

	session, err := h.svc.CreateSession(r.Context(), service.CreateSessionRequest{
		HostUserID:         userID,
		StudySetID:         req.StudySetID,
		QuestionCount:      req.QuestionCount,
		QuestionDurationMs: req.QuestionDurationMs,
		Seed:               req.Seed,
		RequestID:          r.Header.Get("X-Request-ID"),
	})
	if err != nil {
		mapError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, session)
}

// getSession handles GET /v1/live-sessions/{sessionId}
func (h *Handlers) getSession(w http.ResponseWriter, r *http.Request) {
	userID := extractHostUserID(r)
	if userID <= 0 {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	sessionID, err := parseSessionID(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_SESSION_ID", "invalid session id")
		return
	}
	session, err := h.svc.GetSession(r.Context(), sessionID)
	if err != nil {
		mapError(w, r, err)
		return
	}
	if session.HostUserID != userID {
		writeError(w, r, http.StatusForbidden, "FORBIDDEN", "not the host of this session")
		return
	}
	participants, _ := h.svc.GetParticipants(r.Context(), sessionID)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"session":      session,
		"participants": participants,
		"answerCount":  0, // TODO: compute
	})
}

// joinSession handles POST /v1/live-sessions/{code}/join
func (h *Handlers) joinSession(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(strings.TrimSpace(r.PathValue("code")))
	if len(code) != 6 {
		writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "invalid join code")
		return
	}
	var req struct {
		DisplayName string `json:"displayName"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.DisplayName == "" {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "displayName required")
		return
	}

	resp, err := h.svc.JoinSession(r.Context(), service.JoinSessionRequest{
		Code:        code,
		DisplayName: req.DisplayName,
		RequestID:   r.Header.Get("X-Request-ID"),
	})
	if err != nil {
		mapError(w, r, err)
		return
	}
	h.broadcaster.BroadcastEvent(resp.SessionID, fmt.Sprintf("join-%s", resp.ParticipantID), "participant.joined", map[string]interface{}{
		"id": resp.ParticipantID, "displayName": model.NormalizeDisplayName(req.DisplayName),
		"totalScore": 0, "correctCount": 0, "totalResponseTimeMs": 0,
	})
	writeJSON(w, http.StatusOK, resp)
}

// playerState handles GET /v1/live-sessions/{sessionId}/player-state
func (h *Handlers) playerState(w http.ResponseWriter, r *http.Request) {
	sessionID, err := parseSessionID(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_SESSION_ID", "invalid session id")
		return
	}
	token := extractParticipantToken(r)
	participant, err := h.svc.AuthenticateParticipant(r.Context(), token, sessionID)
	if err != nil {
		mapError(w, r, err)
		return
	}
	session, err := h.svc.GetSession(r.Context(), sessionID)
	if err != nil {
		mapError(w, r, err)
		return
	}
	question, _ := h.svc.GetCurrentQuestion(r.Context(), sessionID)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"sessionId":     sessionID,
		"participantId": participant.ID,
		"status":        session.Status,
		"stateVersion":  session.StateVersion,
		"question":      question,
	})
}

// sseStream handles GET /v1/live-sessions/{sessionId}/events
func (h *Handlers) sseStream(w http.ResponseWriter, r *http.Request) {
	sessionID, err := parseSessionID(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_SESSION_ID", "invalid session id")
		return
	}

	// Determine role
	role := "host"
	participantID := ""
	userID := extractHostUserID(r)
	session, err := h.svc.GetSession(r.Context(), sessionID)
	if err != nil {
		mapError(w, r, err)
		return
	}
	if userID > 0 {
		if session.HostUserID != userID {
			writeError(w, r, http.StatusForbidden, "FORBIDDEN", "not the host of this session")
			return
		}
	} else {
		role = "player"
		participant, authErr := h.svc.AuthenticateParticipant(r.Context(), extractParticipantToken(r), sessionID)
		if authErr != nil {
			mapError(w, r, authErr)
			return
		}
		participantID = participant.ID
	}

	client := &realtime.Client{
		ID:            realtime.ClientID(),
		SessionID:     sessionID,
		Role:          role,
		UserID:        userID,
		ParticipantID: participantID,
		LastEventID:   r.Header.Get("Last-Event-ID"),
		Ch:            make(chan []byte, 256),
		Done:          make(chan struct{}),
	}

	// Replay persisted events after the last domain event ID.
	var replay []events.EventEnvelope
	if client.LastEventID != "" {
		stored, replayErr := h.svc.ReplayEvents(r.Context(), sessionID, client.LastEventID)
		if replayErr == nil {
			for _, item := range stored {
				var envelope events.EventEnvelope
				if json.Unmarshal([]byte(item["data"]), &envelope) == nil {
					replay = append(replay, envelope)
				}
			}
		}
	}
	if len(replay) == 0 {
		replay = append(replay, events.EventEnvelope{
			EventID:      fmt.Sprintf("snapshot-%d", session.StateVersion),
			EventType:    "session.snapshot",
			EventVersion: 1,
			AggregateID:  strconv.FormatInt(sessionID, 10),
			OccurredAt:   time.Now().UTC(),
			Data:         map[string]interface{}{"session": session},
		})
	}

	realtime.HandleSSE(w, r, client, h.broadcaster, replay)
}

// startSession handles POST /v1/live-sessions/{sessionId}/start
func (h *Handlers) startSession(w http.ResponseWriter, r *http.Request) {
	userID := extractHostUserID(r)
	if userID <= 0 {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	sessionID, err := parseSessionID(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_SESSION_ID", "invalid session id")
		return
	}
	session, err := h.svc.StartSession(r.Context(), sessionID, userID, r.Header.Get("X-Request-ID"))
	if err != nil {
		mapError(w, r, err)
		return
	}
	if h.scheduler != nil {
		h.scheduler.ScheduleAutoClose(sessionID, session.QuestionDurationMs)
	}
	// Broadcast to connected clients
	h.broadcaster.BroadcastEvent(sessionID, fmt.Sprintf("evt-%d-start", sessionID), "session.started", map[string]interface{}{
		"sessionId": sessionID,
	})
	if len(session.QuestionSnapshot) > 0 {
		q, _ := h.svc.GetCurrentQuestion(r.Context(), sessionID)
		if q == nil {
			writeJSON(w, http.StatusOK, session)
			return
		}
		h.broadcaster.BroadcastEvent(sessionID, fmt.Sprintf("evt-%d-q0", sessionID), "question.opened", map[string]interface{}{
			"index":       q.Index,
			"flashcardId": q.FlashcardID,
			"text":        q.Text,
			"choices":     q.Choices,
			"startsAt":    q.StartsAt,
			"closesAt":    q.ClosesAt,
		})
	}
	writeJSON(w, http.StatusOK, session)
}

// closeQuestion handles POST /v1/live-sessions/{sessionId}/questions/current/close
func (h *Handlers) closeQuestion(w http.ResponseWriter, r *http.Request) {
	userID := extractHostUserID(r)
	if userID <= 0 {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	sessionID, err := parseSessionID(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_SESSION_ID", "invalid session id")
		return
	}
	session, err := h.svc.CloseQuestion(r.Context(), sessionID, userID, r.Header.Get("X-Request-ID"))
	if err != nil {
		mapError(w, r, err)
		return
	}
	if h.scheduler != nil {
		h.scheduler.CancelTimer(sessionID)
	}
	h.broadcaster.BroadcastEvent(sessionID, fmt.Sprintf("evt-%d-close", sessionID), "question.closed", map[string]interface{}{
		"sessionId":     sessionID,
		"questionIndex": session.CurrentQuestionIdx,
	})
	if entries, lbErr := h.svc.GetLeaderboard(r.Context(), sessionID); lbErr == nil {
		h.broadcaster.BroadcastEvent(sessionID, fmt.Sprintf("evt-%d-lb-%d", sessionID, session.StateVersion), "leaderboard.updated", map[string]interface{}{
			"rankings": entries,
		})
	}
	writeJSON(w, http.StatusOK, session)
}

// nextQuestion handles POST /v1/live-sessions/{sessionId}/questions/next
func (h *Handlers) nextQuestion(w http.ResponseWriter, r *http.Request) {
	userID := extractHostUserID(r)
	if userID <= 0 {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	sessionID, err := parseSessionID(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_SESSION_ID", "invalid session id")
		return
	}
	session, err := h.svc.NextQuestion(r.Context(), sessionID, userID, r.Header.Get("X-Request-ID"))
	if err != nil {
		mapError(w, r, err)
		return
	}
	if h.scheduler != nil {
		h.scheduler.ScheduleAutoClose(sessionID, session.QuestionDurationMs)
	}
	if session.CurrentQuestionIdx != nil && *session.CurrentQuestionIdx < len(session.QuestionSnapshot) {
		q, _ := h.svc.GetCurrentQuestion(r.Context(), sessionID)
		if q == nil {
			writeJSON(w, http.StatusOK, session)
			return
		}
		h.broadcaster.BroadcastEvent(sessionID, fmt.Sprintf("evt-%d-q%d", sessionID, q.Index), "question.opened", map[string]interface{}{
			"index":       q.Index,
			"flashcardId": q.FlashcardID,
			"text":        q.Text,
			"choices":     q.Choices,
			"startsAt":    q.StartsAt,
			"closesAt":    q.ClosesAt,
		})
	}
	writeJSON(w, http.StatusOK, session)
}

// submitAnswer handles POST /v1/live-sessions/{sessionId}/answers
func (h *Handlers) submitAnswer(w http.ResponseWriter, r *http.Request) {
	token := extractParticipantToken(r)
	if token == "" {
		writeError(w, r, http.StatusUnauthorized, "PARTICIPANT_TOKEN_INVALID", "participant token required")
		return
	}
	sessionID, err := parseSessionID(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_SESSION_ID", "invalid session id")
		return
	}
	var req struct {
		QuestionIndex int `json:"questionIndex"`
		Answer        struct {
			SelectedIndex *int `json:"selectedIndex"`
		} `json:"answer"`
		IdempotencyKey string `json:"idempotencyKey"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.IdempotencyKey == "" {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "idempotencyKey required")
		return
	}
	if req.Answer.SelectedIndex == nil {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "selectedIndex required")
		return
	}

	participant, err := h.svc.AuthenticateParticipant(r.Context(), token, sessionID)
	if err != nil {
		mapError(w, r, err)
		return
	}
	participantID := participant.ID

	answer, err := h.svc.SubmitAnswer(r.Context(), service.SubmitAnswerRequest{
		SessionID:        sessionID,
		ParticipantID:    participantID,
		ParticipantToken: token,
		QuestionIndex:    req.QuestionIndex,
		Answer: struct {
			SelectedIndex int `json:"selectedIndex"`
		}{SelectedIndex: *req.Answer.SelectedIndex},
		IdempotencyKey: req.IdempotencyKey,
		RequestID:      r.Header.Get("X-Request-ID"),
	})
	if err != nil {
		mapError(w, r, err)
		return
	}

	// Send answer.accepted only to the submitting player
	h.broadcaster.BroadcastToPlayer(sessionID, participantID, fmt.Sprintf("evt-%d-ans-%s", sessionID, participantID), "answer.accepted", map[string]interface{}{
		"accepted":      true,
		"questionIndex": answer.QuestionIndex,
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"accepted":      true,
		"questionIndex": answer.QuestionIndex,
		"submittedAt":   answer.SubmittedAt.Format(time.RFC3339Nano),
	})
}

// getLeaderboard handles GET /v1/live-sessions/{sessionId}/leaderboard
func (h *Handlers) getLeaderboard(w http.ResponseWriter, r *http.Request) {
	sessionID, err := parseSessionID(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_SESSION_ID", "invalid session id")
		return
	}
	session, err := h.svc.GetSession(r.Context(), sessionID)
	if err != nil {
		mapError(w, r, err)
		return
	}
	userID := extractHostUserID(r)
	if userID > 0 {
		if session.HostUserID != userID {
			writeError(w, r, http.StatusForbidden, "FORBIDDEN", "not the host of this session")
			return
		}
	} else if _, err := h.svc.AuthenticateParticipant(r.Context(), extractParticipantToken(r), sessionID); err != nil {
		mapError(w, r, err)
		return
	}
	entries, err := h.svc.GetLeaderboard(r.Context(), sessionID)
	if err != nil {
		mapError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"sessionId":    sessionID,
		"stateVersion": session.StateVersion,
		"rankings":     entries,
	})
}

// endSession handles POST /v1/live-sessions/{sessionId}/end
func (h *Handlers) endSession(w http.ResponseWriter, r *http.Request) {
	userID := extractHostUserID(r)
	if userID <= 0 {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	sessionID, err := parseSessionID(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_SESSION_ID", "invalid session id")
		return
	}
	session, err := h.svc.EndSession(r.Context(), sessionID, userID, r.Header.Get("X-Request-ID"))
	if err != nil {
		mapError(w, r, err)
		return
	}
	if h.scheduler != nil {
		h.scheduler.CancelTimer(sessionID)
	}
	h.broadcaster.BroadcastEvent(sessionID, fmt.Sprintf("evt-%d-end", sessionID), "session.ended", map[string]interface{}{
		"sessionId": sessionID,
	})
	writeJSON(w, http.StatusOK, session)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst interface{}) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return false
	}
	return true
}

func mapError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, model.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "LIVE_SESSION_NOT_FOUND", "live session not found")
	case errors.Is(err, model.ErrForbidden):
		writeError(w, r, http.StatusForbidden, "FORBIDDEN", "not authorized")
	case errors.Is(err, model.ErrUnauthorized):
		writeError(w, r, http.StatusUnauthorized, "PARTICIPANT_TOKEN_INVALID", "participant token invalid")
	case errors.Is(err, model.ErrConflict):
		writeError(w, r, http.StatusConflict, "IDEMPOTENCY_CONFLICT", "idempotency key reused with different payload")
	case errors.Is(err, model.ErrInvalidState):
		writeError(w, r, http.StatusConflict, "LIVE_INVALID_STATE", err.Error())
	case errors.Is(err, model.ErrAlreadyAnswered):
		writeError(w, r, http.StatusConflict, "ANSWER_ALREADY_SUBMITTED", "answer already submitted")
	case errors.Is(err, model.ErrDisplayNameTaken):
		writeError(w, r, http.StatusConflict, "DISPLAY_NAME_TAKEN", "display name already taken")
	case errors.Is(err, model.ErrExpired):
		writeError(w, r, http.StatusGone, "LIVE_SESSION_ENDED", "session has ended")
	case errors.Is(err, model.ErrValidation):
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
	case errors.Is(err, model.ErrStateUnavailable):
		writeError(w, r, http.StatusServiceUnavailable, "LIVE_STATE_UNAVAILABLE", "live state unavailable")
	case errors.Is(err, model.ErrAnswerTooLate):
		writeError(w, r, http.StatusGone, "LIVE_INVALID_STATE", "answer submitted after deadline")
	case errors.Is(err, model.ErrNoMoreQuestions):
		writeError(w, r, http.StatusConflict, "LIVE_INVALID_STATE", "no more questions")
	default:
		log.Printf("[handlers] unhandled error: %v", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}
