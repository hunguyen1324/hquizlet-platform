package model

import "time"

// StudySet represents a collection of flashcards owned by a user.
type StudySet struct {
	ID          int64       `json:"id"`
	UserID      int64       `json:"userId"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	CreatedAt   time.Time   `json:"createdAt"`
	UpdatedAt   time.Time   `json:"updatedAt"`
	Flashcards  []Flashcard `json:"flashcards,omitempty"`
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
	ID          int64      `json:"id"`
	UserID      int64      `json:"userId"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	StudySets   []StudySet `json:"studySets,omitempty"`
}

// CreateFolderInput is the validated payload for creating a folder.
type CreateFolderInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// UpdateFolderInput is the validated payload for updating a folder.
type UpdateFolderInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}
