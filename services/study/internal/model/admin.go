package model

import "time"

// AdminStudySetRow is one row of the platform-wide study-set list (ADM-06).
// It exposes the owner so moderators can see whose content they are looking at.
type AdminStudySetRow struct {
	ID          int64     `json:"id"`
	Hidden      bool      `json:"hidden"`
	Title       string    `json:"title"`
	OwnerUserID int64     `json:"ownerUserId"`
	CardCount   int       `json:"cardCount"`
	QuizCount   int       `json:"quizCount"`
	UpdatedAt   time.Time `json:"updatedAt"`
	CreatedAt   time.Time `json:"createdAt"`
}
