package model

import "time"

// AdminClassRow is one row of the platform-wide class list.
// Only admins (dashboard:ViewAdmin) may read it; it exposes the owner so moderators can act.
type AdminClassRow struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	OwnerUserID   int64     `json:"ownerUserId"`
	MemberCount   int       `json:"memberCount"`
	StudySetCount int       `json:"studySetCount"`
	CreatedAt     time.Time `json:"createdAt"`
}
