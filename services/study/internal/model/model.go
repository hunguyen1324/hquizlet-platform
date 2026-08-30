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
