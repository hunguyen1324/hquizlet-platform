package model

import "time"

// StudySet represents a collection of flashcards owned by a user.
type StudySet struct {
	ID             int64       `json:"id"`
	UserID         int64       `json:"userId"`
	Title          string      `json:"title"`
	Description    string      `json:"description"`
	CreatedAt      time.Time   `json:"createdAt"`
	UpdatedAt      time.Time   `json:"updatedAt"`
	FlashcardCount int         `json:"flashcardCount,omitempty"`
	Flashcards     []Flashcard `json:"flashcards,omitempty"`
}

// Flashcard is a single term/definition card inside a StudySet.
type Flashcard struct {
	ID         int64     `json:"id"`
	StudySetID int64     `json:"studySetId"`
	Term       string    `json:"term"`
	Definition string    `json:"definition"`
	Starred    bool      `json:"starred"`
	Position   int       `json:"position"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// CreateStudySetInput is the validated payload for creating a study set.
type CreateStudySetInput struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// UpdateStudySetInput is the validated payload for updating a study set.
type UpdateStudySetInput struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// CreateFlashcardInput is the validated payload for creating a flashcard.
type CreateFlashcardInput struct {
	Term       string `json:"term"`
	Definition string `json:"definition"`
}

// UpdateFlashcardInput is the validated payload for updating a flashcard.
type UpdateFlashcardInput struct {
	Term       string `json:"term"`
	Definition string `json:"definition"`
}

// BulkFlashcardItem represents one card in a bulk save operation.
// ID == 0 means create; ID > 0 means update; Delete == true means delete.
type BulkFlashcardItem struct {
	ID         int64  `json:"id"`
	Term       string `json:"term"`
	Definition string `json:"definition"`
	Position   int    `json:"position"`
	Delete     bool   `json:"delete"`
}

// BulkSaveFlashcardsInput is the payload for bulk create/update/delete.
type BulkSaveFlashcardsInput struct {
	Cards []BulkFlashcardItem `json:"cards"`
}

// BulkSaveResult summarises what was created/updated/deleted.
type BulkSaveResult struct {
	Created []Flashcard `json:"created"`
	Updated []Flashcard `json:"updated"`
	Deleted []int64     `json:"deleted"`
}

// ---------------------------------------------------------------------------
// Learning Progress domain
// ---------------------------------------------------------------------------

// LearningMode is the set of valid study modes.
type LearningMode string

const (
	ModFlashcards LearningMode = "flashcards"
	ModLearn      LearningMode = "learn"
	ModTest       LearningMode = "test"
	ModMatch      LearningMode = "match"
)

// ValidMode returns true if m is one of the four supported modes.
func (m LearningMode) Valid() bool {
	switch m {
	case ModFlashcards, ModLearn, ModTest, ModMatch:
		return true
	}
	return false
}

// LearningSession is the aggregate root for a single learning attempt.
type LearningSession struct {
	ID             int64                `json:"id"`
	UserID         int64                `json:"userId"`
	StudySetID     int64                `json:"studySetId"`
	Mode           LearningMode         `json:"mode"`
	Score          int                  `json:"score"`
	Total          int                  `json:"total"`
	StartedAt      time.Time            `json:"startedAt"`
	CompletedAt    *time.Time           `json:"completedAt"`
	IdempotencyKey string               `json:"idempotencyKey"`
	CreatedAt      time.Time            `json:"createdAt"`
	CardResults    []LearningCardResult `json:"cardResults,omitempty"`
}

// LearningCardResult records the outcome for a single flashcard within a session.
type LearningCardResult struct {
	ID             int64 `json:"id"`
	SessionID      int64 `json:"sessionId"`
	FlashcardID    int64 `json:"flashcardId"`
	Correct        bool  `json:"correct"`
	Attempts       int   `json:"attempts"`
	ResponseTimeMs *int  `json:"responseTimeMs"`
}

// CardResultInput is the per-card payload submitted by the client.
type CardResultInput struct {
	FlashcardID    int64 `json:"flashcardId"`
	Correct        bool  `json:"correct"`
	Attempts       int   `json:"attempts"`
	ResponseTimeMs *int  `json:"responseTimeMs"`
}

// SaveProgressInput is the validated payload for POST /v1/study-sets/{id}/progress.
// userID is NOT accepted from the client – it is injected by the gateway.
// studySetID comes from the URL path, not from this body.
type SaveProgressInput struct {
	Mode           LearningMode      `json:"mode"`
	Score          int               `json:"score"`
	Total          int               `json:"total"`
	StartedAt      time.Time         `json:"startedAt"`
	CompletedAt    *time.Time        `json:"completedAt"`
	CardResults    []CardResultInput `json:"cardResults"`
	IdempotencyKey string            `json:"idempotencyKey"`
}

// ProgressSummary is the aggregate view returned by GET /v1/study-sets/{id}/progress.
type ProgressSummary struct {
	StudySetID    int64             `json:"studySetId"`
	TotalSessions int               `json:"totalSessions"`
	BestScore     *int              `json:"bestScore"`
	LastMode      *LearningMode     `json:"lastMode"`
	History       []LearningSession `json:"history"`
	Page          int               `json:"page"`
	PerPage       int               `json:"perPage"`
	TotalPages    int               `json:"totalPages"`
}

// ProgressFilter holds pagination params for listing history.
type ProgressFilter struct {
	Page    int // 1-based
	PerPage int // default 20, max 50
}

// ---------------------------------------------------------------------------

// StudySetFilter holds optional search/filter/sort params for listing.
type StudySetFilter struct {
	Search  string // title substring search
	SortBy  string // "updated" | "created" | "title" (default: updated)
	Page    int    // 1-based
	PerPage int    // default 20, max 100
}

// StudySetListResult is a paginated list of study sets.
type StudySetListResult struct {
	Items      []StudySet `json:"items"`
	Total      int        `json:"total"`
	Page       int        `json:"page"`
	PerPage    int        `json:"perPage"`
	TotalPages int        `json:"totalPages"`
}

// Folder is a named container for study sets, owned by a user.
type Folder struct {
	ID            int64      `json:"id"`
	UserID        int64      `json:"-"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	StudySetCount int        `json:"studySetCount"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	StudySets     []StudySet `json:"studySets,omitempty"`
}

// CreateFolderInput is the validated payload for creating a folder.
type CreateFolderInput struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// UpdateFolderInput is the validated payload for updating a folder.
type UpdateFolderInput struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}
