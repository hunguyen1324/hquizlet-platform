package model

import "time"

// QuizSession is persisted only on the server. Never serialize it to a client.
type QuizSession struct {
	ID           string         `json:"id"`
	UserID       int64          `json:"userId"`
	StudySetID   int64          `json:"studySetId"`
	Mode         string         `json:"mode"`
	Instant      bool           `json:"instant"`
	Layout       string         `json:"layout"`
	StartedAt    time.Time      `json:"startedAt"`
	Deadline     *time.Time     `json:"deadline,omitempty"`
	ExpiresAt    time.Time      `json:"expiresAt"`
	SubmittedAt  *time.Time     `json:"submittedAt,omitempty"`
	Items        []SessionItem  `json:"items"`
	Answers      map[int]string `json:"answers"`
	Seen         map[int]bool   `json:"seen"`
	CurrentIndex int            `json:"currentIndex"`
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

type QuizSessionView struct {
	ID           string             `json:"id"`
	Mode         string             `json:"mode"`
	Instant      bool               `json:"instant"`
	Layout       string             `json:"layout"`
	ServerTime   time.Time          `json:"serverTime"`
	Deadline     *time.Time         `json:"deadline,omitempty"`
	Submitted    bool               `json:"submitted"`
	CurrentIndex int                `json:"currentIndex"`
	Manifest     []QuizManifestItem `json:"manifest"`
	Page         []QuizPageItem     `json:"page"`
	Answers      map[int]string     `json:"answers"`
	Results      map[int]bool       `json:"results"`
	Score        *int               `json:"score,omitempty"`
	TimeUsed     int                `json:"timeUsed"`
}
