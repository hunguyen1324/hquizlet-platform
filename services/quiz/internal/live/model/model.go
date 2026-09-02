// Package model defines the Live Quiz domain types and state machine.
// Dev 3 - [P6-GO-01]
package model

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// Session status constants matching the state machine.
const (
	StatusLobby          = "LOBBY"
	StatusQuestionOpen   = "QUESTION_OPEN"
	StatusQuestionClosed = "QUESTION_CLOSED"
	StatusLeaderboard    = "LEADERBOARD"
	StatusEnded          = "ENDED"
)

// Valid states for the state machine.
var validStatuses = map[string]bool{
	StatusLobby:          true,
	StatusQuestionOpen:   true,
	StatusQuestionClosed: true,
	StatusLeaderboard:    true,
	StatusEnded:          true,
}

// Join code alphabet: unambiguous uppercase + digits (no I, O, 0, 1, L).
const joinCodeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

// Session represents a live quiz session.
type Session struct {
	ID                 int64      `json:"id"`
	Code               string     `json:"code"`
	HostUserID         int64      `json:"hostUserId"`
	StudySetID         int64      `json:"studySetId"`
	Status             string     `json:"status"`
	Seed               int64      `json:"seed"`
	QuestionCount      int        `json:"questionCount"`
	QuestionDurationMs int        `json:"questionDurationMs"`
	CurrentQuestionIdx *int       `json:"currentQuestionIndex,omitempty"`
	StateVersion       int64      `json:"stateVersion"`
	QuestionSnapshot   []Question `json:"-"`
	StartedAt          *time.Time `json:"startedAt,omitempty"`
	EndedAt            *time.Time `json:"endedAt,omitempty"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

// Question is a frozen question from the study set snapshot.
type Question struct {
	Index        int      `json:"index"`
	FlashcardID  int64    `json:"flashcardId"`
	Term         string   `json:"term"`
	Definition   string   `json:"definition"`
	Choices      []string `json:"choices,omitempty"`
	CorrectIndex int      `json:"-"`
}

// Participant represents a player in a live session.
type Participant struct {
	ID                  string     `json:"id"`
	LiveSessionID       int64      `json:"liveSessionId"`
	UserID              *int64     `json:"userId,omitempty"`
	DisplayName         string     `json:"displayName"`
	TotalScore          int        `json:"totalScore"`
	CorrectCount        int        `json:"correctCount"`
	TotalResponseTimeMs int64      `json:"totalResponseTimeMs"`
	JoinedAt            time.Time  `json:"joinedAt"`
	LastSeenAt          time.Time  `json:"lastSeenAt"`
	LeftAt              *time.Time `json:"leftAt,omitempty"`
}

// Answer represents a player's answer to a question.
type Answer struct {
	ID              int64     `json:"id"`
	LiveSessionID   int64     `json:"liveSessionId"`
	ParticipantID   string    `json:"participantId"`
	QuestionIndex   int       `json:"questionIndex"`
	FlashcardID     int64     `json:"flashcardId"`
	SubmittedAnswer []byte    `json:"submittedAnswer"`
	IsCorrect       bool      `json:"isCorrect"`
	ScoreAwarded    int       `json:"scoreAwarded"`
	ResponseTimeMs  int       `json:"responseTimeMs"`
	IdempotencyKey  string    `json:"idempotencyKey"`
	SubmittedAt     time.Time `json:"submittedAt"`
}

// LeaderboardEntry is a single row in the leaderboard.
type LeaderboardEntry struct {
	Rank              int    `json:"rank"`
	ParticipantID     string `json:"participantId"`
	DisplayName       string `json:"displayName"`
	TotalScore        int    `json:"totalScore"`
	CorrectCount      int    `json:"correctCount"`
	TotalResponseTime int64  `json:"totalResponseTimeMs"`
}

// Typed errors for the live domain.
var (
	ErrNotFound          = errors.New("live session not found")
	ErrUnauthorized      = errors.New("participant token invalid")
	ErrForbidden         = errors.New("not the host of this session")
	ErrInvalidState      = errors.New("invalid state transition")
	ErrConflict          = errors.New("conflict")
	ErrAlreadyAnswered   = errors.New("answer already submitted for this question")
	ErrDisplayNameTaken  = errors.New("display name already taken in this session")
	ErrExpired           = errors.New("live session has ended")
	ErrValidation        = errors.New("validation error")
	ErrStateUnavailable  = errors.New("live state unavailable")
	ErrDependencyUnavail = errors.New("dependency unavailable")
	ErrNoMoreQuestions   = errors.New("no more questions available")
	ErrAnswerTooLate     = errors.New("answer submitted after deadline")
	ErrMaxParticipants   = errors.New("maximum participants reached")
)

// StateTransition validates whether a command can execute in the current state.
func StateTransition(currentStatus, command string) (string, error) {
	switch command {
	case "start":
		if currentStatus == StatusLobby {
			return StatusQuestionOpen, nil
		}
	case "close":
		if currentStatus == StatusQuestionOpen {
			return StatusQuestionClosed, nil
		}
		// close is idempotent in other non-terminal states
		if currentStatus == StatusQuestionClosed || currentStatus == StatusLeaderboard {
			return currentStatus, nil
		}
	case "next":
		if currentStatus == StatusQuestionClosed || currentStatus == StatusLeaderboard {
			return StatusQuestionOpen, nil
		}
	case "leaderboard":
		if currentStatus == StatusQuestionClosed {
			return StatusLeaderboard, nil
		}
	case "end":
		if currentStatus != StatusEnded {
			return StatusEnded, nil
		}
		return StatusEnded, nil
	}
	return "", fmt.Errorf("%w: cannot %s from %s", ErrInvalidState, command, currentStatus)
}

// GenerateJoinCode creates a cryptographically random 6-char join code.
func GenerateJoinCode() (string, error) {
	code := make([]byte, 6)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(joinCodeAlphabet))))
		if err != nil {
			return "", fmt.Errorf("generate join code: %w", err)
		}
		code[i] = joinCodeAlphabet[n.Int64()]
	}
	return string(code), nil
}

// GenerateParticipantToken creates an opaque participant token.
func GenerateParticipantToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate participant token: %w", err)
	}
	return fmt.Sprintf("%x", b), nil
}

// GenerateEventID returns an RFC 4122 version-4 UUID without an extra dependency.
func GenerateEventID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate event id: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// NormalizeDisplayName trims and collapses whitespace for uniqueness checks.
func NormalizeDisplayName(name string) string {
	s := strings.TrimSpace(name)
	s = strings.Join(strings.Fields(s), " ")
	return s
}

// ScoreAnswer computes the score for a correct/incorrect answer with time bonus.
// baseScore = 1000 if correct, 0 if incorrect
// timeBonus = floor(500 * remainingMs / questionDurationMs)
// questionScore = baseScore + clamp(timeBonus, 0, 500)
func ScoreAnswer(isCorrect bool, remainingMs, questionDurationMs int) int {
	if !isCorrect {
		return 0
	}
	base := 1000
	bonus := 0
	if questionDurationMs > 0 && remainingMs > 0 {
		bonus = (500 * remainingMs) / questionDurationMs
		if bonus > 500 {
			bonus = 500
		}
		if bonus < 0 {
			bonus = 0
		}
	}
	return base + bonus
}

// LeaderboardSort sorts participants by totalScore desc, correctCount desc, totalResponseTimeMs asc, joinedAt asc.
func LeaderboardSort(entries []LeaderboardEntry) {
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			a, b := entries[i], entries[j]
			if a.TotalScore < b.TotalScore ||
				(a.TotalScore == b.TotalScore && a.CorrectCount < b.CorrectCount) ||
				(a.TotalScore == b.TotalScore && a.CorrectCount == b.CorrectCount && a.TotalResponseTime > b.TotalResponseTime) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
	for i := range entries {
		entries[i].Rank = i + 1
	}
}
