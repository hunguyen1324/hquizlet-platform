// Package model defines the domain types for the class service.
package model

import (
	"encoding/json"
	"time"
)

// Class represents a class created by a teacher/owner.
type Class struct {
	ID           int64     `json:"id"`
	OwnerUserID  int64     `json:"ownerUserId"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	InviteCode   string    `json:"inviteCode"`
	MaxMembers   int       `json:"maxMembers"`
	MemberCount  int       `json:"memberCount"`
	StudySetCount int      `json:"studySetCount"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// ClassSummary is a lightweight view for class list.
type ClassSummary struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	InviteCode    string    `json:"inviteCode"`
	MemberCount   int       `json:"memberCount"`
	StudySetCount int       `json:"studySetCount"`
	MyRole        string    `json:"myRole"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// ClassDetail includes extra fields for single class view.
type ClassDetail struct {
	ClassSummary
	MaxMembers int `json:"maxMembers"`
}

// ClassMember represents a user's membership in a class.
type ClassMember struct {
	ID       int64     `json:"id"`
	ClassID  int64     `json:"classId"`
	UserID   int64     `json:"userId"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joinedAt"`
}

// ClassStudySet represents a study set assigned to a class.
type ClassStudySet struct {
	ClassID      int64     `json:"classId"`
	StudySetID   int64     `json:"studySetId"`
	Title        string    `json:"title,omitempty"`
	FlashcardCount int     `json:"flashcardCount,omitempty"`
	AddedByUserID int64    `json:"addedByUserId"`
	AddedAt      time.Time `json:"addedAt"`
}

// ActivityEvent represents an event in the activity feed.
type ActivityEvent struct {
	ID         int64           `json:"id"`
	UserID     int64           `json:"userId"`
	EventType  string          `json:"eventType"`
	EntityType string          `json:"entityType"`
	EntityID   *int64          `json:"entityId,omitempty"`
	ClassID    *int64          `json:"classId,omitempty"`
	Metadata   json.RawMessage `json:"metadata,omitempty"`
	OccurredAt time.Time       `json:"occurredAt"`
}

// ActivityItem is the API response format for activity feed items.
type ActivityItem struct {
	ID         int64            `json:"id"`
	EventType  string           `json:"eventType"`
	EntityType string           `json:"entityType"`
	EntityID   *int64           `json:"entityId,omitempty"`
	ClassID    *int64           `json:"classId,omitempty"`
	Metadata   map[string]any   `json:"metadata,omitempty"`
	OccurredAt time.Time        `json:"occurredAt"`
}

// ActivityFeedResponse is the paginated activity feed response.
type ActivityFeedResponse struct {
	Items      []ActivityItem `json:"items"`
	NextCursor string         `json:"nextCursor,omitempty"`
	HasMore    bool           `json:"hasMore"`
}

// ActivityCursor is the decoded cursor for pagination.
type ActivityCursor struct {
	Timestamp time.Time
	ID        int64
}

// --- Input types ---

// CreateClassInput is the validated payload for creating a class.
type CreateClassInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	MaxMembers  int    `json:"maxMembers"`
}

// UpdateClassInput is the validated payload for updating a class.
type UpdateClassInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// AddMemberRequest is the payload for adding a member manually.
type AddMemberRequest struct {
	UserID int64  `json:"userId"`
	Role   string `json:"role"`
}

// UpdateMemberRoleRequest is the payload for changing a member's role.
type UpdateMemberRoleRequest struct {
	Role string `json:"role"`
}

// AddStudySetRequest is the payload for assigning a study set to a class.
type AddStudySetRequest struct {
	StudySetID int64 `json:"studySetId"`
}

// JoinClassResponse is returned when a user joins a class.
type JoinClassResponse struct {
	ClassID   int64     `json:"classId"`
	ClassName string    `json:"className"`
	MyRole    string    `json:"myRole"`
	JoinedAt  time.Time `json:"joinedAt"`
}

// ValidRoles lists the allowed member roles.
var ValidRoles = map[string]bool{
	"owner":   true,
	"teacher": true,
	"student": true,
}
