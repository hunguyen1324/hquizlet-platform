package model

import "time"

// ClassDashboard is the payload for GET /v1/classes/{id}/dashboard.
// The shape is the same for every viewer but the contents differ: teachers receive
// class-wide rows, students receive only their own progress (plus a leaderboard when the
// class has it enabled).
type ClassDashboard struct {
	ClassID         int64                 `json:"classId"`
	ClassName       string                `json:"className"`
	ViewerRole      string                `json:"viewerRole"`
	CanViewAll      bool                  `json:"canViewAll"`
	ShowLeaderboard bool                  `json:"showLeaderboard"`
	MemberCount     int                   `json:"memberCount"`
	ActiveMembers7d int                   `json:"activeMembers7d"`
	AssignedSets    []DashboardSetStat    `json:"assignedSets"`
	Students        []DashboardStudentRow `json:"students"`
	MyProgress      *DashboardStudentRow  `json:"myProgress,omitempty"`
	RecentActivity  []ActivityItem        `json:"recentActivity"`
}

// DashboardSetStat aggregates one assigned study set across the class.
type DashboardSetStat struct {
	DueAt         *time.Time `json:"dueAt,omitempty"`
	StudySetID    int64      `json:"studySetId"`
	Title         string     `json:"title"`
	Sessions      int        `json:"sessions"`
	Learners      int        `json:"learners"`
	CompletionPct int        `json:"completionPct"`
	AvgScorePct   int        `json:"avgScorePct"`
}

// DashboardStudentRow aggregates one member's progress.
type DashboardStudentRow struct {
	UserID         int64   `json:"userId"`
	Name           string  `json:"name"`
	Email          string  `json:"email,omitempty"`
	Role           string  `json:"role,omitempty"`
	Sessions       int     `json:"sessions"`
	CompletedSets  int     `json:"completedSets"`
	AvgScorePct    int     `json:"avgScorePct"`
	LastActivityAt *string `json:"lastActivityAt"`
}
