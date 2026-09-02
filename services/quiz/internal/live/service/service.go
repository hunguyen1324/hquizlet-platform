// Package service implements the Live Quiz business logic.
// Dev 3 - [P6-GO-01, P6-GO-02, P6-GO-03, P6-GO-04]
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/quiz/internal/engine"
	"github.com/hunguyen1324/hquizlet-platform/services/quiz/internal/live/events"
	"github.com/hunguyen1324/hquizlet-platform/services/quiz/internal/live/model"
	"github.com/hunguyen1324/hquizlet-platform/services/quiz/internal/live/redisstore"
	"github.com/hunguyen1324/hquizlet-platform/services/quiz/internal/live/repository"
	"github.com/hunguyen1324/hquizlet-platform/services/quiz/internal/studyclient"
	"github.com/lib/pq"
)

// EventPublisher is the interface for publishing domain events.
type EventPublisher interface {
	Publish(ctx context.Context, subject string, event interface{}) error
	IsConnected() bool
}

// Service coordinates live quiz operations.
type Service struct {
	repo   *repository.DB
	redis  *redisstore.Store
	study  *studyclient.Client
	events EventPublisher
}

// New creates a new live service.
func New(repo *repository.DB, redis *redisstore.Store, study *studyclient.Client, pub EventPublisher) *Service {
	return &Service{repo: repo, redis: redis, study: study, events: pub}
}

// CreateSessionRequest is the input for creating a live session.
type CreateSessionRequest struct {
	HostUserID         int64  `json:"hostUserId"`
	StudySetID         int64  `json:"studySetId"`
	QuestionCount      int    `json:"questionCount"`
	QuestionDurationMs int    `json:"questionDurationMs"`
	Seed               int64  `json:"seed"`
	RequestID          string `json:"requestId"`
}

// CreateSession creates a new live session lobby.
func (s *Service) CreateSession(ctx context.Context, req CreateSessionRequest) (*model.Session, error) {
	// Verify host owns the study set
	set, err := s.study.GetFlashcards(ctx, req.StudySetID, req.HostUserID)
	if err != nil {
		return nil, fmt.Errorf("verify ownership: %w", err)
	}
	if len(set.Flashcards) < 1 {
		return nil, fmt.Errorf("%w: study set has no flashcards", model.ErrValidation)
	}
	if req.QuestionCount < 1 || req.QuestionCount > len(set.Flashcards) {
		return nil, fmt.Errorf("%w: question count exceeds available flashcards", model.ErrValidation)
	}

	code, err := model.GenerateJoinCode()
	if err != nil {
		return nil, err
	}

	// Freeze question snapshot using deterministic engine
	seed := req.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	items, err := engine.Generate(set.Flashcards, "test", uint64(seed), req.QuestionCount)
	if err != nil {
		return nil, fmt.Errorf("generate questions: %w", err)
	}

	// Build question snapshot
	questions := make([]model.Question, 0, len(items))
	for i, item := range items {
		q := model.Question{
			Index:       i,
			FlashcardID: item.FlashcardID,
			Term:        item.Term,
			Definition:  item.Definition,
			Choices:     item.Choices,
		}
		// Find correct index in choices
		for ci, c := range item.Choices {
			if c == item.Definition {
				q.CorrectIndex = ci
				break
			}
		}
		questions = append(questions, q)
	}

	snapshotJSON, _ := json.Marshal(questions)

	session := &model.Session{
		Code:               code,
		HostUserID:         req.HostUserID,
		StudySetID:         req.StudySetID,
		Status:             model.StatusLobby,
		Seed:               seed,
		QuestionCount:      req.QuestionCount,
		QuestionDurationMs: req.QuestionDurationMs,
		StateVersion:       1,
		QuestionSnapshot:   questions,
	}

	if err := s.repo.CreateSession(ctx, session, snapshotJSON); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	// Write to Redis
	if err := s.redis.SetSessionState(ctx, &redisstore.SessionState{
		SessionID:          session.ID,
		Code:               code,
		Status:             model.StatusLobby,
		StateVersion:       1,
		HostUserID:         req.HostUserID,
		QuestionCount:      req.QuestionCount,
		QuestionDurationMs: req.QuestionDurationMs,
	}); err != nil {
		log.Printf("[service] redis set state failed: %v", err)
	}
	if err := s.redis.SetCodeMapping(ctx, code, session.ID); err != nil {
		log.Printf("[service] redis set code mapping failed: %v", err)
	}

	// Publish event
	s.publishEvent(ctx, session.ID, events.SubjectSessionCreated, "live.session.created", 1, req.RequestID, map[string]interface{}{
		"sessionId":  session.ID,
		"code":       code,
		"hostUserId": req.HostUserID,
	})

	return session, nil
}

// JoinSessionRequest is the input for joining a session.
type JoinSessionRequest struct {
	Code        string `json:"code"`
	DisplayName string `json:"displayName"`
	RequestID   string `json:"requestId"`
}

// JoinSessionResponse is returned when a player joins.
type JoinSessionResponse struct {
	SessionID        int64  `json:"sessionId"`
	ParticipantID    string `json:"participantId"`
	ParticipantToken string `json:"participantToken"`
	Status           string `json:"status"`
}

// JoinSession lets a player join by code.
func (s *Service) JoinSession(ctx context.Context, req JoinSessionRequest) (*JoinSessionResponse, error) {
	session, err := s.repo.GetSessionByCode(ctx, req.Code)
	if err != nil {
		return nil, err
	}
	if session.Status == model.StatusEnded {
		return nil, model.ErrExpired
	}
	if session.Status != model.StatusLobby {
		return nil, model.ErrInvalidState
	}

	normalized := model.NormalizeDisplayName(req.DisplayName)
	if normalized == "" || len([]rune(normalized)) > 40 {
		return nil, fmt.Errorf("%w: display name must be 1-40 characters", model.ErrValidation)
	}

	token, err := model.GenerateParticipantToken()
	if err != nil {
		return nil, err
	}
	tokenHash := hashToken(token)

	p := &model.Participant{
		LiveSessionID: session.ID,
		DisplayName:   normalized,
	}
	if err := s.repo.CreateParticipant(ctx, p, tokenHash); err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return nil, model.ErrDisplayNameTaken
		}
		return nil, fmt.Errorf("create participant: %w", err)
	}

	// Redis
	s.redis.AddParticipant(ctx, session.ID, redisstore.ParticipantInfo{
		ID:          p.ID,
		DisplayName: normalized,
	})
	s.redis.SetPresence(ctx, session.ID, p.ID)

	// Publish event
	s.publishEvent(ctx, session.ID, events.SubjectParticipantJoined, "live.participant.joined", 1, req.RequestID, map[string]interface{}{
		"participantId": p.ID,
		"displayName":   normalized,
	})

	return &JoinSessionResponse{
		SessionID:        session.ID,
		ParticipantID:    p.ID,
		ParticipantToken: token,
		Status:           session.Status,
	}, nil
}

// StartSession starts a live session (host only).
func (s *Service) StartSession(ctx context.Context, sessionID, hostUserID int64, requestID string) (*model.Session, error) {
	session, err := s.repo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session.HostUserID != hostUserID {
		return nil, model.ErrForbidden
	}
	if len(session.QuestionSnapshot) == 0 {
		return nil, fmt.Errorf("%w: no questions available", model.ErrValidation)
	}

	newStatus, err := model.StateTransition(session.Status, "start")
	if err != nil {
		return nil, err
	}

	idx := 0
	updated, err := s.repo.UpdateSessionCAS(ctx, sessionID, session.StateVersion, newStatus, &idx, false)
	if err != nil {
		return nil, err
	}

	// Update Redis
	s.redis.SetSessionState(ctx, &redisstore.SessionState{
		SessionID:          sessionID,
		Code:               session.Code,
		Status:             newStatus,
		CurrentQuestionIdx: 0,
		StateVersion:       updated.StateVersion,
		HostUserID:         hostUserID,
		QuestionCount:      session.QuestionCount,
		QuestionDurationMs: session.QuestionDurationMs,
	})

	if len(session.QuestionSnapshot) > 0 {
		q := session.QuestionSnapshot[0]
		now := time.Now().UTC()
		closesAt := now.Add(time.Duration(session.QuestionDurationMs) * time.Millisecond)
		s.redis.SetCurrentQuestion(ctx, sessionID, &redisstore.QuestionState{
			Index:       q.Index,
			FlashcardID: q.FlashcardID,
			Text:        q.Term,
			Choices:     q.Choices,
			StartsAt:    now.Format(time.RFC3339Nano),
			ClosesAt:    closesAt.Format(time.RFC3339Nano),
		})
	}

	s.publishEvent(ctx, sessionID, events.SubjectSessionStarted, "live.session.started", 1, requestID, map[string]interface{}{
		"sessionId": sessionID,
	})
	s.publishEvent(ctx, sessionID, events.SubjectQuestionOpened, "live.question.opened", 1, requestID, map[string]interface{}{
		"sessionId":     sessionID,
		"questionIndex": 0,
	})

	return updated, nil
}

// CloseQuestion closes the current question (host only).
func (s *Service) CloseQuestion(ctx context.Context, sessionID, hostUserID int64, requestID string) (*model.Session, error) {
	session, err := s.repo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session.HostUserID != hostUserID {
		return nil, model.ErrForbidden
	}
	if session.Status == model.StatusQuestionClosed || session.Status == model.StatusLeaderboard {
		return session, nil
	}

	newStatus, err := model.StateTransition(session.Status, "close")
	if err != nil {
		return nil, err
	}

	// Finalize scores for this question
	_ = s.repo.FinalizeScores(ctx, sessionID)

	updated, err := s.repo.UpdateSessionCAS(ctx, sessionID, session.StateVersion, newStatus, session.CurrentQuestionIdx, false)
	if err != nil {
		return nil, err
	}
	cqi := 0
	if updated.CurrentQuestionIdx != nil {
		cqi = *updated.CurrentQuestionIdx
	}
	s.redis.SetSessionState(ctx, &redisstore.SessionState{
		SessionID:          sessionID,
		Code:               session.Code,
		Status:             newStatus,
		CurrentQuestionIdx: cqi,
		StateVersion:       updated.StateVersion,
		HostUserID:         hostUserID,
		QuestionCount:      session.QuestionCount,
		QuestionDurationMs: session.QuestionDurationMs,
	})

	s.publishEvent(ctx, sessionID, events.SubjectQuestionClosed, "live.question.closed", 1, requestID, map[string]interface{}{
		"sessionId":     sessionID,
		"questionIndex": session.CurrentQuestionIdx,
	})

	return updated, nil
}

// NextQuestion opens the next question (host only).
func (s *Service) NextQuestion(ctx context.Context, sessionID, hostUserID int64, requestID string) (*model.Session, error) {
	session, err := s.repo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session.HostUserID != hostUserID {
		return nil, model.ErrForbidden
	}

	newStatus, err := model.StateTransition(session.Status, "next")
	if err != nil {
		return nil, err
	}

	nextIdx := 0
	if session.CurrentQuestionIdx != nil {
		nextIdx = *session.CurrentQuestionIdx + 1
	}
	if nextIdx >= len(session.QuestionSnapshot) {
		return nil, model.ErrNoMoreQuestions
	}

	updated, err := s.repo.UpdateSessionCAS(ctx, sessionID, session.StateVersion, newStatus, &nextIdx, false)
	if err != nil {
		return nil, err
	}

	s.redis.SetSessionState(ctx, &redisstore.SessionState{
		SessionID:          sessionID,
		Code:               session.Code,
		Status:             newStatus,
		CurrentQuestionIdx: nextIdx,
		StateVersion:       updated.StateVersion,
		HostUserID:         hostUserID,
		QuestionCount:      session.QuestionCount,
		QuestionDurationMs: session.QuestionDurationMs,
	})

	q := session.QuestionSnapshot[nextIdx]
	now := time.Now().UTC()
	closesAt := now.Add(time.Duration(session.QuestionDurationMs) * time.Millisecond)
	s.redis.SetCurrentQuestion(ctx, sessionID, &redisstore.QuestionState{
		Index:       q.Index,
		FlashcardID: q.FlashcardID,
		Text:        q.Term,
		Choices:     q.Choices,
		StartsAt:    now.Format(time.RFC3339Nano),
		ClosesAt:    closesAt.Format(time.RFC3339Nano),
	})

	s.publishEvent(ctx, sessionID, events.SubjectQuestionOpened, "live.question.opened", 1, requestID, map[string]interface{}{
		"sessionId":     sessionID,
		"questionIndex": nextIdx,
	})

	return updated, nil
}

// SubmitAnswerRequest is the input for submitting an answer.
type SubmitAnswerRequest struct {
	SessionID        int64
	ParticipantID    string
	ParticipantToken string
	QuestionIndex    int
	Answer           struct {
		SelectedIndex int `json:"selectedIndex"`
	}
	IdempotencyKey string
	RequestID      string
}

// SubmitAnswer submits a player's answer.
func (s *Service) SubmitAnswer(ctx context.Context, req SubmitAnswerRequest) (*model.Answer, error) {
	participant, err := s.AuthenticateParticipant(ctx, req.ParticipantToken, req.SessionID)
	if err != nil {
		return nil, err
	}
	if req.ParticipantID != "" && req.ParticipantID != participant.ID {
		return nil, model.ErrForbidden
	}
	req.ParticipantID = participant.ID
	if req.IdempotencyKey == "" || len(req.IdempotencyKey) > 128 {
		return nil, model.ErrValidation
	}
	answerJSON, err := json.Marshal(req.Answer)
	if err != nil {
		return nil, model.ErrValidation
	}
	if existing, lookupErr := s.repo.GetAnswerByIdempotencyKey(ctx, req.ParticipantID, req.IdempotencyKey); lookupErr == nil {
		if existing.LiveSessionID == req.SessionID && existing.QuestionIndex == req.QuestionIndex &&
			string(existing.SubmittedAnswer) == string(answerJSON) {
			return existing, nil
		}
		return nil, model.ErrConflict
	}

	session, err := s.repo.GetSession(ctx, req.SessionID)
	if err != nil {
		return nil, err
	}

	// Validate state
	if session.Status != model.StatusQuestionOpen {
		return nil, model.ErrInvalidState
	}
	if session.CurrentQuestionIdx == nil || *session.CurrentQuestionIdx != req.QuestionIndex {
		return nil, model.ErrInvalidState
	}

	// Check deadline via Redis question state
	qState, stateErr := s.redis.GetCurrentQuestion(ctx, req.SessionID)
	if stateErr != nil || qState == nil {
		return nil, model.ErrStateUnavailable
	}
	closesAt, err := time.Parse(time.RFC3339Nano, qState.ClosesAt)
	if err != nil {
		return nil, model.ErrStateUnavailable
	}
	if time.Now().After(closesAt) {
		return nil, model.ErrAnswerTooLate
	}

	// Find correct answer from snapshot
	if req.QuestionIndex >= len(session.QuestionSnapshot) {
		return nil, model.ErrValidation
	}
	q := session.QuestionSnapshot[req.QuestionIndex]
	if req.Answer.SelectedIndex < 0 || req.Answer.SelectedIndex >= len(q.Choices) {
		return nil, model.ErrValidation
	}
	isCorrect := req.Answer.SelectedIndex == q.CorrectIndex
	remainingMs := 0
	remaining := time.Until(closesAt)
	if remaining > 0 {
		remainingMs = int(remaining.Milliseconds())
	}
	score := model.ScoreAnswer(isCorrect, remainingMs, session.QuestionDurationMs)

	answer := &model.Answer{
		LiveSessionID:  req.SessionID,
		ParticipantID:  req.ParticipantID,
		QuestionIndex:  req.QuestionIndex,
		FlashcardID:    q.FlashcardID,
		IsCorrect:      isCorrect,
		ScoreAwarded:   score,
		ResponseTimeMs: session.QuestionDurationMs - remainingMs,
		IdempotencyKey: req.IdempotencyKey,
	}

	// Persist answer + update participant scores in a transaction
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	err = tx.QueryRowContext(ctx, `
		INSERT INTO live_session_answers (live_session_id, participant_id, question_index, flashcard_id,
		                                  submitted_answer, is_correct, score_awarded, response_time_ms, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, submitted_at
	`, req.SessionID, req.ParticipantID, req.QuestionIndex, q.FlashcardID,
		answerJSON, isCorrect, score, answer.ResponseTimeMs, req.IdempotencyKey,
	).Scan(&answer.ID, &answer.SubmittedAt)

	if err != nil {
		existing, lookupErr := s.repo.GetAnswerByIdempotencyKey(ctx, req.ParticipantID, req.IdempotencyKey)
		if lookupErr == nil {
			if existing.LiveSessionID == req.SessionID && existing.QuestionIndex == req.QuestionIndex &&
				string(existing.SubmittedAnswer) == string(answerJSON) {
				return existing, nil
			}
			return nil, model.ErrConflict
		}
		return nil, model.ErrAlreadyAnswered
	}

	// Update participant totals
	correctDelta := 0
	if isCorrect {
		correctDelta = 1
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE live_session_participants
		SET total_score = total_score + $2, correct_count = correct_count + $3, total_response_time_ms = total_response_time_ms + $4
		WHERE id = $1
	`, req.ParticipantID, score, correctDelta, answer.ResponseTimeMs)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// Update Redis leaderboard
	s.redis.UpdateLeaderboardEntry(ctx, req.SessionID, req.ParticipantID, "", score, correctDelta, int64(answer.ResponseTimeMs))

	s.publishEvent(ctx, req.SessionID, events.SubjectAnswerSubmitted, "live.answer.submitted", 1, req.RequestID, map[string]interface{}{
		"sessionId":     req.SessionID,
		"participantId": req.ParticipantID,
		"questionIndex": req.QuestionIndex,
		"accepted":      true,
	})

	return answer, nil
}

// EndSession ends a live session (host only).
func (s *Service) EndSession(ctx context.Context, sessionID, hostUserID int64, requestID string) (*model.Session, error) {
	session, err := s.repo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session.HostUserID != hostUserID {
		return nil, model.ErrForbidden
	}
	if session.Status == model.StatusEnded {
		return session, nil
	}

	newStatus, err := model.StateTransition(session.Status, "end")
	if err != nil {
		return nil, err
	}

	// Finalize scores
	_ = s.repo.FinalizeScores(ctx, sessionID)

	updated, err := s.repo.UpdateSessionCAS(ctx, sessionID, session.StateVersion, newStatus, session.CurrentQuestionIdx, true)
	if err != nil {
		return nil, err
	}

	s.publishEvent(ctx, sessionID, events.SubjectSessionEnded, "live.session.ended", 1, requestID, map[string]interface{}{
		"sessionId": sessionID,
	})

	return updated, nil
}

// GetSession retrieves a session by ID.
func (s *Service) GetSession(ctx context.Context, id int64) (*model.Session, error) {
	return s.repo.GetSession(ctx, id)
}

// GetLeaderboard retrieves the leaderboard for a session.
func (s *Service) GetLeaderboard(ctx context.Context, sessionID int64) ([]model.LeaderboardEntry, error) {
	return s.repo.ListLeaderboard(ctx, sessionID)
}

// GetParticipants lists all participants for a session.
func (s *Service) GetParticipants(ctx context.Context, sessionID int64) ([]model.Participant, error) {
	return s.repo.ListParticipants(ctx, sessionID)
}

// AuthenticateParticipant verifies an opaque participant token and scopes it
// to the requested live session.
func (s *Service) AuthenticateParticipant(ctx context.Context, token string, sessionID int64) (*model.Participant, error) {
	if token == "" {
		return nil, model.ErrUnauthorized
	}
	p, err := s.repo.GetParticipantByTokenHash(ctx, hashToken(token))
	if err != nil {
		if err == model.ErrNotFound {
			return nil, model.ErrUnauthorized
		}
		return nil, err
	}
	if p.LiveSessionID != sessionID || p.LeftAt != nil {
		return nil, model.ErrForbidden
	}
	return p, nil
}

// GetCurrentQuestion returns the public, answer-free current question state.
func (s *Service) GetCurrentQuestion(ctx context.Context, sessionID int64) (*redisstore.QuestionState, error) {
	q, err := s.redis.GetCurrentQuestion(ctx, sessionID)
	if err != nil {
		return nil, model.ErrStateUnavailable
	}
	return q, nil
}

// ReplayEvents returns persisted realtime events after a Redis stream ID.
func (s *Service) ReplayEvents(ctx context.Context, sessionID int64, afterID string) ([]map[string]string, error) {
	return s.redis.ReplayEvents(ctx, sessionID, afterID)
}

// RecoverSessions rebuilds Redis state for non-terminal sessions after restart.
func (s *Service) RecoverSessions(ctx context.Context) ([]*model.Session, error) {
	sessions, err := s.repo.RecoverNonTerminalSessions(ctx)
	if err != nil {
		return nil, fmt.Errorf("recover sessions: %w", err)
	}
	for _, sess := range sessions {
		state := &redisstore.SessionState{
			SessionID:          sess.ID,
			Code:               sess.Code,
			Status:             sess.Status,
			StateVersion:       sess.StateVersion,
			HostUserID:         sess.HostUserID,
			QuestionCount:      sess.QuestionCount,
			QuestionDurationMs: sess.QuestionDurationMs,
		}
		if sess.CurrentQuestionIdx != nil {
			state.CurrentQuestionIdx = *sess.CurrentQuestionIdx
		}
		_ = s.redis.SetSessionState(ctx, state)
		_ = s.redis.SetCodeMapping(ctx, sess.Code, sess.ID)
		if sess.Status == model.StatusQuestionOpen && sess.CurrentQuestionIdx != nil &&
			*sess.CurrentQuestionIdx >= 0 && *sess.CurrentQuestionIdx < len(sess.QuestionSnapshot) {
			q := sess.QuestionSnapshot[*sess.CurrentQuestionIdx]
			startsAt := sess.UpdatedAt
			closesAt := startsAt.Add(time.Duration(sess.QuestionDurationMs) * time.Millisecond)
			_ = s.redis.SetCurrentQuestion(ctx, sess.ID, &redisstore.QuestionState{
				Index: q.Index, FlashcardID: q.FlashcardID, Text: q.Term, Choices: q.Choices,
				StartsAt: startsAt.Format(time.RFC3339Nano), ClosesAt: closesAt.Format(time.RFC3339Nano),
			})
		}
		log.Printf("[service] recovered session %d (status=%s)", sess.ID, sess.Status)
	}
	return sessions, nil
}

func (s *Service) publishEvent(ctx context.Context, sessionID int64, subject, eventType string, version int, requestID string, data interface{}) {
	eventID, err := model.GenerateEventID()
	if err != nil {
		log.Printf("[service] generate event id failed: %v", err)
		return
	}
	env := events.EventEnvelope{
		EventID:      eventID,
		EventType:    eventType,
		EventVersion: version,
		AggregateID:  fmt.Sprintf("%d", sessionID),
		OccurredAt:   time.Now().UTC(),
		RequestID:    requestID,
		Data:         data,
	}
	payload, _ := json.Marshal(env)
	if err := s.repo.CreateOutboxEvent(ctx, nil, env.EventID, sessionID, subject, version, payload); err != nil {
		log.Printf("[service] create outbox event failed: %v", err)
	}
	if err := s.redis.PublishEvent(ctx, sessionID, env.EventID, payload); err != nil {
		log.Printf("[service] persist realtime event failed: %v", err)
	}
}

// hashToken creates a SHA-256 hash of the participant token.
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
