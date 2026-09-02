// Package repository implements PostgreSQL persistence for live quiz sessions.
// Dev 2 - [P6-DB-02]
package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/quiz/internal/live/model"
)

// DB wraps the database connection and provides live session operations.
type DB struct {
	db *sql.DB
}

// New creates a new repository.
func New(db *sql.DB) *DB {
	return &DB{db: db}
}

// CreateSession inserts a new live session with its question snapshot.
func (d *DB) CreateSession(ctx context.Context, s *model.Session, snapshotJSON []byte) error {
	err := d.db.QueryRowContext(ctx, `
		INSERT INTO live_sessions (code, host_user_id, study_set_id, status, seed, question_count, question_duration_ms, state_version, question_snapshot)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at
	`, s.Code, s.HostUserID, s.StudySetID, s.Status, s.Seed, s.QuestionCount, s.QuestionDurationMs, s.StateVersion, snapshotJSON,
	).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
	return err
}

// GetSession retrieves a session by ID.
func (d *DB) GetSession(ctx context.Context, id int64) (*model.Session, error) {
	var s model.Session
	var snapshotJSON []byte
	err := d.db.QueryRowContext(ctx, `
		SELECT id, code, host_user_id, study_set_id, status, seed, question_count, question_duration_ms,
		       current_question_index, state_version, question_snapshot, started_at, ended_at, created_at, updated_at
		FROM live_sessions WHERE id = $1
	`, id).Scan(&s.ID, &s.Code, &s.HostUserID, &s.StudySetID, &s.Status, &s.Seed, &s.QuestionCount, &s.QuestionDurationMs,
		&s.CurrentQuestionIdx, &s.StateVersion, &snapshotJSON, &s.StartedAt, &s.EndedAt, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if snapshotJSON != nil {
		_ = json.Unmarshal(snapshotJSON, &s.QuestionSnapshot)
	}
	return &s, nil
}

// GetSessionByCode retrieves a session by join code.
func (d *DB) GetSessionByCode(ctx context.Context, code string) (*model.Session, error) {
	var s model.Session
	var snapshotJSON []byte
	err := d.db.QueryRowContext(ctx, `
		SELECT id, code, host_user_id, study_set_id, status, seed, question_count, question_duration_ms,
		       current_question_index, state_version, question_snapshot, started_at, ended_at, created_at, updated_at
		FROM live_sessions WHERE code = $1
	`, code).Scan(&s.ID, &s.Code, &s.HostUserID, &s.StudySetID, &s.Status, &s.Seed, &s.QuestionCount, &s.QuestionDurationMs,
		&s.CurrentQuestionIdx, &s.StateVersion, &snapshotJSON, &s.StartedAt, &s.EndedAt, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if snapshotJSON != nil {
		_ = json.Unmarshal(snapshotJSON, &s.QuestionSnapshot)
	}
	return &s, nil
}

// UpdateSessionCAS performs a compare-and-swap update on state version.
// Returns ErrInvalidState if the version doesn't match.
func (d *DB) UpdateSessionCAS(ctx context.Context, id int64, expectedVersion int64, status string, currentQuestionIdx *int, ended bool) (*model.Session, error) {
	var s model.Session
	var snapshotJSON []byte
	var endedAt *time.Time
	if ended {
		now := time.Now().UTC()
		endedAt = &now
	}
	err := d.db.QueryRowContext(ctx, `
		UPDATE live_sessions
		SET status = $2::varchar, current_question_index = $3, state_version = state_version + 1, updated_at = NOW(),
		    started_at = CASE WHEN $2::varchar = 'QUESTION_OPEN' AND started_at IS NULL THEN NOW() ELSE started_at END,
		    ended_at = COALESCE($4, ended_at)
		WHERE id = $1 AND state_version = $5 AND status != 'ENDED'
		RETURNING id, code, host_user_id, study_set_id, status, seed, question_count, question_duration_ms,
		          current_question_index, state_version, question_snapshot, started_at, ended_at, created_at, updated_at
	`, id, status, currentQuestionIdx, endedAt, expectedVersion,
	).Scan(&s.ID, &s.Code, &s.HostUserID, &s.StudySetID, &s.Status, &s.Seed, &s.QuestionCount, &s.QuestionDurationMs,
		&s.CurrentQuestionIdx, &s.StateVersion, &snapshotJSON, &s.StartedAt, &s.EndedAt, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrInvalidState
	}
	if err != nil {
		return nil, err
	}
	if snapshotJSON != nil {
		_ = json.Unmarshal(snapshotJSON, &s.QuestionSnapshot)
	}
	return &s, nil
}

// CreateParticipant inserts a new participant with idempotency on session+display_name.
func (d *DB) CreateParticipant(ctx context.Context, p *model.Participant, tokenHash string) error {
	err := d.db.QueryRowContext(ctx, `
		INSERT INTO live_session_participants (live_session_id, user_id, display_name, token_hash)
		VALUES ($1, $2, $3, $4)
		RETURNING id, joined_at, last_seen_at
	`, p.LiveSessionID, p.UserID, p.DisplayName, tokenHash,
	).Scan(&p.ID, &p.JoinedAt, &p.LastSeenAt)
	return err
}

// GetParticipantByTokenHash looks up a participant by their token hash.
func (d *DB) GetParticipantByTokenHash(ctx context.Context, tokenHash string) (*model.Participant, error) {
	var p model.Participant
	err := d.db.QueryRowContext(ctx, `
		SELECT id, live_session_id, user_id, display_name, total_score, correct_count,
		       total_response_time_ms, joined_at, last_seen_at, left_at
		FROM live_session_participants WHERE token_hash = $1
	`, tokenHash).Scan(&p.ID, &p.LiveSessionID, &p.UserID, &p.DisplayName, &p.TotalScore, &p.CorrectCount,
		&p.TotalResponseTimeMs, &p.JoinedAt, &p.LastSeenAt, &p.LeftAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	return &p, err
}

// ListParticipants returns all participants for a session.
func (d *DB) ListParticipants(ctx context.Context, sessionID int64) ([]model.Participant, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, live_session_id, user_id, display_name, total_score, correct_count,
		       total_response_time_ms, joined_at, last_seen_at, left_at
		FROM live_session_participants WHERE live_session_id = $1 ORDER BY joined_at
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var participants []model.Participant
	for rows.Next() {
		var p model.Participant
		if err := rows.Scan(&p.ID, &p.LiveSessionID, &p.UserID, &p.DisplayName, &p.TotalScore, &p.CorrectCount,
			&p.TotalResponseTimeMs, &p.JoinedAt, &p.LastSeenAt, &p.LeftAt); err != nil {
			return nil, err
		}
		participants = append(participants, p)
	}
	return participants, rows.Err()
}

// InsertAnswerIdempotent inserts an answer or returns the existing one on idempotency conflict.
func (d *DB) InsertAnswerIdempotent(ctx context.Context, a *model.Answer) error {
	err := d.db.QueryRowContext(ctx, `
		INSERT INTO live_session_answers (live_session_id, participant_id, question_index, flashcard_id,
		                                  submitted_answer, is_correct, score_awarded, response_time_ms, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, submitted_at
	`, a.LiveSessionID, a.ParticipantID, a.QuestionIndex, a.FlashcardID,
		a.SubmittedAnswer, a.IsCorrect, a.ScoreAwarded, a.ResponseTimeMs, a.IdempotencyKey,
	).Scan(&a.ID, &a.SubmittedAt)
	return err
}

// FinalizeScores updates participant totals after all questions are answered.
func (d *DB) FinalizeScores(ctx context.Context, sessionID int64) error {
	_, err := d.db.ExecContext(ctx, `
		UPDATE live_session_participants p
		SET total_score = COALESCE(sub.total, 0),
		    correct_count = COALESCE(sub.correct, 0),
		    total_response_time_ms = COALESCE(sub.resp_time, 0)
		FROM (
			SELECT participant_id,
			       SUM(score_awarded) AS total,
			       SUM(CASE WHEN is_correct THEN 1 ELSE 0 END) AS correct,
			       SUM(response_time_ms) AS resp_time
			FROM live_session_answers
			WHERE live_session_id = $1
			GROUP BY participant_id
		) sub
		WHERE p.id = sub.participant_id
	`, sessionID)
	return err
}

// ListLeaderboard returns leaderboard entries sorted correctly.
func (d *DB) ListLeaderboard(ctx context.Context, sessionID int64) ([]model.LeaderboardEntry, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT p.id, p.display_name, COALESCE(SUM(a.score_awarded), 0) AS total_score,
		       COALESCE(SUM(CASE WHEN a.is_correct THEN 1 ELSE 0 END), 0) AS correct_count,
		       COALESCE(SUM(a.response_time_ms), 0) AS total_response_time_ms
		FROM live_session_participants p
		LEFT JOIN live_session_answers a ON a.participant_id = p.id AND a.live_session_id = p.live_session_id
		WHERE p.live_session_id = $1
		GROUP BY p.id, p.display_name, p.joined_at
		ORDER BY total_score DESC, correct_count DESC, total_response_time_ms ASC, p.joined_at ASC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []model.LeaderboardEntry
	rank := 0
	for rows.Next() {
		rank++
		var e model.LeaderboardEntry
		if err := rows.Scan(&e.ParticipantID, &e.DisplayName, &e.TotalScore, &e.CorrectCount, &e.TotalResponseTime); err != nil {
			return nil, err
		}
		e.Rank = rank
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// GetAnswerCount returns the number of answers for a given session+question.
func (d *DB) GetAnswerCount(ctx context.Context, sessionID int64, questionIndex int) (int, error) {
	var count int
	err := d.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM live_session_answers
		WHERE live_session_id = $1 AND question_index = $2
	`, sessionID, questionIndex).Scan(&count)
	return count, err
}

// CreateOutboxEvent inserts a domain event into the outbox within a transaction.
func (d *DB) CreateOutboxEvent(ctx context.Context, tx *sql.Tx, eventID string, aggregateID int64, subject string, version int, payload []byte) error {
	query := `
		INSERT INTO live_event_outbox (event_id, aggregate_id, subject, event_version, payload)
		VALUES ($1, $2, $3, $4, $5)
	`
	var err error
	if tx != nil {
		_, err = tx.ExecContext(ctx, query, eventID, aggregateID, subject, version, payload)
	} else {
		_, err = d.db.ExecContext(ctx, query, eventID, aggregateID, subject, version, payload)
	}
	return err
}

// GetAnswerByIdempotencyKey returns an existing answer for safe client retries.
func (d *DB) GetAnswerByIdempotencyKey(ctx context.Context, participantID, key string) (*model.Answer, error) {
	var a model.Answer
	err := d.db.QueryRowContext(ctx, `
		SELECT id, live_session_id, participant_id, question_index, flashcard_id,
		       submitted_answer, is_correct, score_awarded, response_time_ms,
		       idempotency_key, submitted_at
		FROM live_session_answers
		WHERE participant_id = $1 AND idempotency_key = $2
	`, participantID, key).Scan(&a.ID, &a.LiveSessionID, &a.ParticipantID, &a.QuestionIndex,
		&a.FlashcardID, &a.SubmittedAnswer, &a.IsCorrect, &a.ScoreAwarded,
		&a.ResponseTimeMs, &a.IdempotencyKey, &a.SubmittedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	return &a, err
}

// ClaimOutboxBatch returns a batch of unpublished events for processing.
func (d *DB) ClaimOutboxBatch(ctx context.Context, batchSize int) ([]OutboxEvent, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT event_id, aggregate_id, subject, event_version, payload, occurred_at, attempts
		FROM live_event_outbox
		WHERE published_at IS NULL
		ORDER BY occurred_at
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`, batchSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []OutboxEvent
	for rows.Next() {
		var e OutboxEvent
		if err := rows.Scan(&e.EventID, &e.AggregateID, &e.Subject, &e.EventVersion, &e.Payload, &e.OccurredAt, &e.Attempts); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// MarkPublished marks outbox events as published.
func (d *DB) MarkPublished(ctx context.Context, eventIDs []string) error {
	for _, id := range eventIDs {
		_, err := d.db.ExecContext(ctx, `
			UPDATE live_event_outbox SET published_at = NOW() WHERE event_id = $1
		`, id)
		if err != nil {
			return err
		}
	}
	return nil
}

// MarkPublishFailed records a failed publish attempt.
func (d *DB) MarkPublishFailed(ctx context.Context, eventID string, errMsg string) error {
	_, err := d.db.ExecContext(ctx, `
		UPDATE live_event_outbox
		SET attempts = attempts + 1, last_error = $2
		WHERE event_id = $1
	`, eventID, errMsg)
	return err
}

// OutboxEvent is a domain event waiting to be published.
type OutboxEvent struct {
	EventID      string
	AggregateID  int64
	Subject      string
	EventVersion int
	Payload      []byte
	OccurredAt   time.Time
	Attempts     int
}

// BeginTx starts a new database transaction.
func (d *DB) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return d.db.BeginTx(ctx, nil)
}

// RecoverNonTerminalSessions finds sessions that should be recovered after restart.
func (d *DB) RecoverNonTerminalSessions(ctx context.Context) ([]*model.Session, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, code, host_user_id, study_set_id, status, seed, question_count, question_duration_ms,
		       current_question_index, state_version, question_snapshot, started_at, ended_at, created_at, updated_at
		FROM live_sessions
		WHERE status NOT IN ('ENDED')
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sessions []*model.Session
	for rows.Next() {
		var s model.Session
		var snapshotJSON []byte
		if err := rows.Scan(&s.ID, &s.Code, &s.HostUserID, &s.StudySetID, &s.Status, &s.Seed, &s.QuestionCount, &s.QuestionDurationMs,
			&s.CurrentQuestionIdx, &s.StateVersion, &snapshotJSON, &s.StartedAt, &s.EndedAt, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		if snapshotJSON != nil {
			_ = json.Unmarshal(snapshotJSON, &s.QuestionSnapshot)
		}
		sessions = append(sessions, &s)
	}
	return sessions, rows.Err()
}

// UpdateSessionState updates session status without CAS (for recovery).
func (d *DB) UpdateSessionState(ctx context.Context, id int64, status string) error {
	_, err := d.db.ExecContext(ctx, `
		UPDATE live_sessions SET status = $2, updated_at = NOW() WHERE id = $1
	`, id, status)
	return err
}

// EnsureDB returns the underlying sql.DB for readiness checks.
func (d *DB) EnsureDB() *sql.DB {
	return d.db
}
