package model

import "time"

// QuizSession is persisted only on the server. Never serialize it to a client.
type QuizSession struct {
	ID             string     `json:"id"`
	UserID         int64      `json:"userId"`
	StudySetID     int64      `json:"studySetId"`
	Mode           string     `json:"mode"`
	Instant        bool       `json:"instant"`
	Layout         string     `json:"layout"`
	StartedAt      time.Time  `json:"startedAt"`
	Deadline       *time.Time `json:"deadline,omitempty"`
	ExpiresAt      time.Time  `json:"expiresAt"`
	SubmittedAt    *time.Time `json:"submittedAt,omitempty"`
	CompletedScore *int       `json:"completedScore,omitempty"`
	// Items is only populated by legacy sessions (created before snapshots).
	// New sessions keep their questions in an immutable QuizSnapshot and leave
	// this empty, so the mutable state stays a few KB.
	Items        []SessionItem  `json:"items,omitempty"`
	Answers      map[int]string `json:"answers"`
	Seen         map[int]bool   `json:"seen"`
	Total        int            `json:"total,omitempty"`
	SnapshotID   string         `json:"snapshotId,omitempty"`
	CurrentIndex int            `json:"currentIndex"`
	// Sanitized records that Items already hold safe HTML, so read paths can
	// skip re-running the sanitizer. Sessions created before this flag existed
	// stay on the sanitizing path until they expire.
	Sanitized bool `json:"sanitized,omitempty"`
}

// QuestionCount is the number of questions in the session: snapshot-backed
// sessions keep only the count, legacy sessions keep the items themselves.
func (s *QuizSession) QuestionCount() int {
	if len(s.Items) > 0 {
		return len(s.Items)
	}
	return s.Total
}

// QuizSnapshot is the immutable, sanitized question content of one quiz
// version. Many sessions share one snapshot, it is cached in RAM by the
// server, and it never holds learner state.
//
// Pages[i] lists the item indexes served when the player asks for the page
// anchored at item i. Grouped layouts (TOEIC) put a whole passage in one page;
// the flat layout always serves a single item.
type QuizSnapshot struct {
	ID         string             `json:"id"`
	StudySetID int64              `json:"studySetId"`
	Part       int                `json:"part"`
	CreatedAt  time.Time          `json:"createdAt"`
	Items      []SessionItem      `json:"items"`
	Manifest   []QuizManifestItem `json:"manifest"`
	Pages      [][]int            `json:"pages"`
}

type SessionItem struct {
	Number   int          `json:"number"`
	Part     int          `json:"part"`
	Group    int          `json:"group"`
	Question QuizQuestion `json:"question"`
}

type StartQuizInput struct {
	Mode            string `json:"mode"`
	Instant         bool   `json:"instant"`
	Layout          string `json:"layout"`
	Part            int    `json:"part"`
	DurationSeconds int    `json:"durationSeconds"`
}

type QuizManifestItem struct {
	Number int `json:"number"`
	Part   int `json:"part"`
	Group  int `json:"group"`
}

type QuizPageItem struct {
	Index    int          `json:"index"`
	Question QuizQuestion `json:"question"`
	HasAudio bool         `json:"hasAudio"`
	Revealed bool         `json:"revealed"`
}

// QuizPeekView is the read-only payload used to pre-warm the next pages.
// It carries page content only: no answers, results, score or audio rights.
type QuizPeekView struct {
	ID           string         `json:"id"`
	ServerTime   time.Time      `json:"serverTime"`
	CurrentIndex int            `json:"currentIndex"`
	Page         []QuizPageItem `json:"page"`
}

// QuizSessionView is the payload of start/page/answer/submit.
//
// Only start (and an explicit resume with ?manifest=1) carries the manifest.
// Answers and results are page-scoped while the session is open, and complete
// once it is submitted, because the result screen needs every question. The
// client merges each response into the state it already has.
type QuizSessionView struct {
	ID           string             `json:"id"`
	Mode         string             `json:"mode"`
	Instant      bool               `json:"instant"`
	Layout       string             `json:"layout"`
	ServerTime   time.Time          `json:"serverTime"`
	Deadline     *time.Time         `json:"deadline,omitempty"`
	Submitted    bool               `json:"submitted"`
	CurrentIndex int                `json:"currentIndex"`
	Total        int                `json:"total"`
	Answered     int                `json:"answered"`
	Manifest     []QuizManifestItem `json:"manifest,omitempty"`
	Page         []QuizPageItem     `json:"page"`
	Answers      map[int]string     `json:"answers"`
	Results      map[int]bool       `json:"results"`
	Score        *int               `json:"score,omitempty"`
	TimeUsed     int                `json:"timeUsed"`
}
