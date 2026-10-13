package model

// StudyStats is the aggregate payload behind GET /v1/me/study-stats.
// It powers both the student dashboard (streak, continue learning, recent results)
// and the teacher dashboard (owned study set usage).
type StudyStats struct {
	RangeDays       int            `json:"rangeDays"`
	StreakDays      int            `json:"streakDays"`
	StudiedToday    bool           `json:"studiedToday"`
	SessionsTotal   int            `json:"sessionsTotal"`
	SessionsInRange int            `json:"sessionsInRange"`
	CardsInRange    int            `json:"cardsInRange"`
	CorrectInRange  int            `json:"correctInRange"`
	AccuracyPct     int            `json:"accuracyPct"`
	DueCards        int            `json:"dueCards"`
	Series          []StudyDayPoint `json:"series"`
	RecentSessions  []RecentSession `json:"recentSessions"`
	ContinueLearning []ContinueItem `json:"continueLearning"`
	OwnedSets       []OwnedSetStat  `json:"ownedSets"`
}

// StudyDayPoint is one day of the dashboard time series.
type StudyDayPoint struct {
	Date     string `json:"date"` // YYYY-MM-DD
	Sessions int    `json:"sessions"`
	Cards    int    `json:"cards"`
	Correct  int    `json:"correct"`
}

// RecentSession is a completed learning session shown in "kết quả gần đây".
type RecentSession struct {
	StudySetID    int64   `json:"studySetId"`
	StudySetTitle string  `json:"studySetTitle"`
	Mode          string  `json:"mode"`
	Score         int     `json:"score"`
	Total         int     `json:"total"`
	CompletedAt   *string `json:"completedAt"`
	CreatedAt     string  `json:"createdAt"`
}

// ContinueItem is a study set the user has started and can resume.
type ContinueItem struct {
	StudySetID   int64  `json:"studySetId"`
	Title        string `json:"title"`
	LastMode     string `json:"lastMode"`
	Sessions     int    `json:"sessions"`
	BestScore    int    `json:"bestScore"`
	LastPlayedAt string `json:"lastPlayedAt"`
}

// OwnedSetStat summarises usage of a study set owned by the caller.
type OwnedSetStat struct {
	StudySetID  int64  `json:"studySetId"`
	Title       string `json:"title"`
	Sessions    int    `json:"sessions"`
	Learners    int    `json:"learners"`
	AvgScorePct int    `json:"avgScorePct"`
}

// ProgressSummaryRow is one (user, study set) aggregate served to the class dashboard
// through the internal API. It deliberately exposes no card-level detail.
type ProgressSummaryRow struct {
	UserID       int64   `json:"userId"`
	StudySetID   int64   `json:"studySetId"`
	Sessions     int     `json:"sessions"`
	BestScore    int     `json:"bestScore"`
	AvgScorePct  int     `json:"avgScorePct"`
	Completed    bool    `json:"completed"`
	LastActivity *string `json:"lastActivityAt"`
}
