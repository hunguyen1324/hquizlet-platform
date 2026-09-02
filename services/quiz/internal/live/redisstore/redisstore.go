// Package redisstore manages Redis-backed live state, presence, and leaderboard.
// Dev 2 - [P6-REDIS-01]
package redisstore

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Key namespace constants.
const (
	prefix       = "live:v1"
	stateTTL     = 24 * time.Hour
	presenceTTL  = 30 * time.Minute
	replayMaxLen = 1000
)

// Store manages Redis state for live sessions.
type Store struct {
	rdb *redis.Client
}

// New creates a new Redis-backed store.
func New(rdb *redis.Client) *Store {
	return &Store{rdb: rdb}
}

func sessionPrefix(sessionID int64) string {
	return fmt.Sprintf("%s:session:%d", prefix, sessionID)
}

func keyState(sessionID int64) string {
	return sessionPrefix(sessionID) + ":state"
}

func keyParticipants(sessionID int64) string {
	return sessionPrefix(sessionID) + ":participants"
}

func keyPresence(sessionID int64) string {
	return sessionPrefix(sessionID) + ":presence"
}

func keyLeaderboard(sessionID int64) string {
	return sessionPrefix(sessionID) + ":leaderboard"
}

func keyQuestion(sessionID int64) string {
	return sessionPrefix(sessionID) + ":question"
}

func keyCode(code string) string {
	return fmt.Sprintf("%s:code:%s", prefix, code)
}

func keyEvents(sessionID int64) string {
	return sessionPrefix(sessionID) + ":events"
}

// SessionState is the Redis-stored session state.
type SessionState struct {
	SessionID          int64  `json:"sessionId"`
	Code               string `json:"code"`
	Status             string `json:"status"`
	CurrentQuestionIdx int    `json:"currentQuestionIndex"`
	StateVersion       int64  `json:"stateVersion"`
	HostUserID         int64  `json:"hostUserId"`
	QuestionCount      int    `json:"questionCount"`
	QuestionDurationMs int    `json:"questionDurationMs"`
	StartedAt          string `json:"startedAt,omitempty"`
}

// ParticipantInfo for the participants set.
type ParticipantInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

// LeaderboardEntry for the leaderboard sorted set.
type LeaderboardEntry struct {
	ParticipantID string  `json:"participantId"`
	DisplayName   string  `json:"displayName"`
	Score         float64 `json:"score"`
	CorrectCount  int     `json:"correctCount"`
	ResponseTime  int64   `json:"responseTime"`
}

// QuestionState for the current question hash.
type QuestionState struct {
	Index       int      `json:"index"`
	FlashcardID int64    `json:"flashcardId"`
	Text        string   `json:"text"`
	Choices     []string `json:"choices"`
	StartsAt    string   `json:"startsAt"`
	ClosesAt    string   `json:"closesAt"`
}

// SetSessionState stores session state with TTL.
func (s *Store) SetSessionState(ctx context.Context, state *SessionState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, keyState(state.SessionID), data, stateTTL).Err()
}

// GetSessionState retrieves session state.
func (s *Store) GetSessionState(ctx context.Context, sessionID int64) (*SessionState, error) {
	data, err := s.rdb.Get(ctx, keyState(sessionID)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var state SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// SetCodeMapping stores join code -> session ID mapping.
func (s *Store) SetCodeMapping(ctx context.Context, code string, sessionID int64) error {
	return s.rdb.Set(ctx, keyCode(code), sessionID, stateTTL).Err()
}

// GetSessionByCode resolves a join code to a session ID.
func (s *Store) GetSessionByCode(ctx context.Context, code string) (int64, error) {
	val, err := s.rdb.Get(ctx, keyCode(code)).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

// AddParticipant adds a participant to the session set.
func (s *Store) AddParticipant(ctx context.Context, sessionID int64, p ParticipantInfo) error {
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	pipe := s.rdb.Pipeline()
	pipe.SAdd(ctx, keyParticipants(sessionID), data)
	pipe.Expire(ctx, keyParticipants(sessionID), stateTTL)
	_, err = pipe.Exec(ctx)
	return err
}

// ListParticipants returns all participants in the session.
func (s *Store) ListParticipants(ctx context.Context, sessionID int64) ([]ParticipantInfo, error) {
	members, err := s.rdb.SMembers(ctx, keyParticipants(sessionID)).Result()
	if err != nil {
		return nil, err
	}
	var participants []ParticipantInfo
	for _, m := range members {
		var p ParticipantInfo
		if err := json.Unmarshal([]byte(m), &p); err != nil {
			continue
		}
		participants = append(participants, p)
	}
	return participants, nil
}

// SetPresence records participant presence timestamp.
func (s *Store) SetPresence(ctx context.Context, sessionID int64, participantID string) error {
	now := time.Now().UnixMilli()
	return s.rdb.ZAdd(ctx, keyPresence(sessionID), redis.Z{
		Score:  float64(now),
		Member: participantID,
	}).Err()
}

// SetLeaderboard stores the full leaderboard as a sorted set.
func (s *Store) SetLeaderboard(ctx context.Context, sessionID int64, entries []LeaderboardEntry) error {
	pipe := s.rdb.Pipeline()
	key := keyLeaderboard(sessionID)
	pipe.Del(ctx, key)
	for _, e := range entries {
		data, _ := json.Marshal(e)
		// Use negative score for descending sort (higher score = lower number = first)
		pipe.ZAdd(ctx, key, redis.Z{
			Score:  -e.Score,
			Member: data,
		})
	}
	pipe.Expire(ctx, key, stateTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// SetCurrentQuestion stores the current question state.
func (s *Store) SetCurrentQuestion(ctx context.Context, sessionID int64, q *QuestionState) error {
	data, err := json.Marshal(q)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, keyQuestion(sessionID), data, stateTTL).Err()
}

// GetCurrentQuestion retrieves the current question state.
func (s *Store) GetCurrentQuestion(ctx context.Context, sessionID int64) (*QuestionState, error) {
	data, err := s.rdb.Get(ctx, keyQuestion(sessionID)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var q QuestionState
	if err := json.Unmarshal(data, &q); err != nil {
		return nil, err
	}
	return &q, nil
}

// PublishEvent adds an event to the session event stream for SSE replay.
func (s *Store) PublishEvent(ctx context.Context, sessionID int64, eventID string, data []byte) error {
	return s.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: keyEvents(sessionID),
		MaxLen: replayMaxLen,
		Approx: true,
		ID:     "*",
		Values: map[string]interface{}{
			"eventId": eventID,
			"data":    string(data),
		},
	}).Err()
}

// ReplayEvents returns events after a given ID for SSE reconnect.
func (s *Store) ReplayEvents(ctx context.Context, sessionID int64, afterID string) ([]map[string]string, error) {
	events, err := s.rdb.XRangeN(ctx, keyEvents(sessionID), "-", "+", int64(replayMaxLen)).Result()
	if err != nil {
		return nil, err
	}
	var result []map[string]string
	foundLast := afterID == ""
	for _, e := range events {
		entry := map[string]string{"id": e.ID}
		for k, v := range e.Values {
			if s, ok := v.(string); ok {
				entry[k] = s
			}
		}
		if !foundLast {
			if entry["eventId"] == afterID {
				foundLast = true
			}
			continue
		}
		result = append(result, entry)
	}
	// A missing domain event ID means the bounded replay window has a gap. The
	// caller will send a fresh snapshot instead of replaying stale partial data.
	if afterID != "" && !foundLast {
		return nil, nil
	}
	return result, nil
}

// DeleteSession removes all Redis keys for a session.
func (s *Store) DeleteSession(ctx context.Context, sessionID int64, code string) error {
	pipe := s.rdb.Pipeline()
	pipe.Del(ctx, keyState(sessionID))
	pipe.Del(ctx, keyParticipants(sessionID))
	pipe.Del(ctx, keyPresence(sessionID))
	pipe.Del(ctx, keyLeaderboard(sessionID))
	pipe.Del(ctx, keyQuestion(sessionID))
	pipe.Del(ctx, keyEvents(sessionID))
	pipe.Del(ctx, keyCode(code))
	_, err := pipe.Exec(ctx)
	return err
}

// RebuildFromState rebuilds Redis projection from a session state.
func (s *Store) RebuildFromState(ctx context.Context, state *SessionState) error {
	if state == nil {
		return nil
	}
	return s.SetSessionState(ctx, state)
}

// IsHealthy checks Redis connectivity.
func (s *Store) IsHealthy(ctx context.Context) error {
	return s.rdb.Ping(ctx).Err()
}

// SetTTL sets expiration on a key.
func (s *Store) SetTTL(ctx context.Context, key string, ttl time.Duration) error {
	return s.rdb.Expire(ctx, key, ttl).Err()
}

// atomicScoreUpdate performs a Lua-scripted leaderboard update for answer scoring.
var atomicScoreUpdateScript = redis.NewScript(`
local key = KEYS[1]
local participantID = ARGV[1]
local displayName = ARGV[2]
local addScore = tonumber(ARGV[3])
local correctDelta = tonumber(ARGV[4])
local respTimeDelta = tonumber(ARGV[5])

-- Get existing entry
local existing = redis.call('HGET', key, participantID)
local totalScore = addScore
local correctCount = correctDelta
local totalRespTime = respTimeDelta

if existing then
    local data = cjson.decode(existing)
    totalScore = totalScore + (data.score or 0)
    correctCount = correctCount + (data.correctCount or 0)
    totalRespTime = totalRespTime + (data.responseTime or 0)
end

local entry = cjson.encode({
    participantId = participantID,
    displayName = displayName,
    score = totalScore,
    correctCount = correctCount,
    responseTime = totalRespTime
})

redis.call('HSET', key, participantID, entry)
return totalScore
`)

// UpdateLeaderboardEntry atomically updates a participant's score in the leaderboard hash.
func (s *Store) UpdateLeaderboardEntry(ctx context.Context, sessionID int64, participantID, displayName string, addScore int, correctDelta int, respTimeDelta int64) (float64, error) {
	result, err := atomicScoreUpdateScript.Run(ctx, s.rdb, []string{keyLeaderboard(sessionID) + ":hash"},
		participantID, displayName, addScore, correctDelta, respTimeDelta).Float64()
	return result, err
}

// GetLeaderboardFromHash returns leaderboard entries from the hash for sorted display.
func (s *Store) GetLeaderboardFromHash(ctx context.Context, sessionID int64) ([]LeaderboardEntry, error) {
	data, err := s.rdb.HGetAll(ctx, keyLeaderboard(sessionID)+":hash").Result()
	if err != nil {
		return nil, err
	}
	var entries []LeaderboardEntry
	for _, v := range data {
		var e LeaderboardEntry
		if err := json.Unmarshal([]byte(v), &e); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// GetLeaderboardRank returns a participant's rank from the leaderboard hash.
func (s *Store) GetLeaderboardRank(ctx context.Context, sessionID int64, participantID string) (int, error) {
	entries, err := s.GetLeaderboardFromHash(ctx, sessionID)
	if err != nil {
		return 0, err
	}
	// Sort entries
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].Score < entries[j].Score ||
				(entries[i].Score == entries[j].Score && entries[i].CorrectCount < entries[j].CorrectCount) ||
				(entries[i].Score == entries[j].Score && entries[i].CorrectCount == entries[j].CorrectCount && entries[i].ResponseTime > entries[j].ResponseTime) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
	for i, e := range entries {
		if e.ParticipantID == participantID {
			return i + 1, nil
		}
	}
	return 0, nil
}

// SessionCount returns the number of active sessions (for metrics).
func (s *Store) SessionCount(ctx context.Context) (int64, error) {
	keys, err := s.rdb.Keys(ctx, prefix+":session:*:state").Result()
	if err != nil {
		return 0, err
	}
	return int64(len(keys)), nil
}

// IncrCounter increments a counter key.
func (s *Store) IncrCounter(ctx context.Context, name string) error {
	return s.rdb.Incr(ctx, prefix+":counter:"+name).Err()
}

// GetCounter gets a counter value.
func (s *Store) GetCounter(ctx context.Context, name string) (int64, error) {
	val, err := s.rdb.Get(ctx, prefix+":counter:"+name).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

func init() {
	// ensure strconv is used
	_ = strconv.Itoa
}
