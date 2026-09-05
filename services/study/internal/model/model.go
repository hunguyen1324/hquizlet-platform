package model

import (
	"encoding/json"
	"time"
)

// StudySet represents a collection of flashcards owned by a user.
type StudySet struct {
	ID                 int64             `json:"id"`
	UserID             int64             `json:"userId"`
	Title              string            `json:"title"`
	Description        string            `json:"description"`
	ThumbnailURL       *string           `json:"thumbnailUrl,omitempty"`
	ContentType        string            `json:"contentType"`
	TermLanguage       string            `json:"termLanguage"`
	DefinitionLanguage string            `json:"definitionLanguage"`
	Visibility         string            `json:"visibility"`
	CreatedAt          time.Time         `json:"createdAt"`
	UpdatedAt          time.Time         `json:"updatedAt"`
	FlashcardCount     int               `json:"flashcardCount"`
	Flashcards         []Flashcard       `json:"flashcards,omitempty"`
	QuizQuestions      []QuizQuestion    `json:"quizQuestions,omitempty"`
}

// Flashcard is a single term/definition card inside a StudySet.
type Flashcard struct {
	ID              int64     `json:"id"`
	StudySetID      int64     `json:"studySetId"`
	Term            string    `json:"term"`
	Definition      string    `json:"definition"`
	ExampleSentence string    `json:"exampleSentence"`
	HintExplanation string    `json:"hintExplanation"`
	Synonyms        string    `json:"synonyms"`
	ImageURL        *string   `json:"imageUrl,omitempty"`
	Starred         bool      `json:"starred"`
	Position        int       `json:"position"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// CreateStudySetInput is the validated payload for creating a study set.
type CreateStudySetInput struct {
	Title              string `json:"title"`
	Description        string `json:"description"`
	ContentType        string `json:"contentType"`
	TermLanguage       string `json:"termLanguage"`
	DefinitionLanguage string `json:"definitionLanguage"`
	Visibility         string `json:"visibility"`
}

// UpdateStudySetInput is the validated payload for updating a study set.
type UpdateStudySetInput struct {
	Title              string  `json:"title"`
	Description        string  `json:"description"`
	ThumbnailURL       *string `json:"thumbnailUrl,omitempty"`
	TermLanguage       string  `json:"termLanguage"`
	DefinitionLanguage string  `json:"definitionLanguage"`
	Visibility         string  `json:"visibility"`
}

// CreateFlashcardInput is the validated payload for creating a flashcard.
type CreateFlashcardInput struct {
	Term            string  `json:"term"`
	Definition      string  `json:"definition"`
	ExampleSentence string  `json:"exampleSentence"`
	HintExplanation string  `json:"hintExplanation"`
	Synonyms        string  `json:"synonyms"`
	ImageURL        *string `json:"imageUrl,omitempty"`
}

// UpdateFlashcardInput is the validated payload for updating a flashcard.
type UpdateFlashcardInput struct {
	Term            string  `json:"term"`
	Definition      string  `json:"definition"`
	ExampleSentence string  `json:"exampleSentence"`
	HintExplanation string  `json:"hintExplanation"`
	Synonyms        string  `json:"synonyms"`
	ImageURL        *string `json:"imageUrl,omitempty"`
}

// BulkFlashcardItem represents one card in a bulk save operation.
// ID == 0 means create; ID > 0 means update; Delete == true means delete.
type BulkFlashcardItem struct {
	ID              int64   `json:"id"`
	Term            string  `json:"term"`
	Definition      string  `json:"definition"`
	ExampleSentence string  `json:"exampleSentence"`
	HintExplanation string  `json:"hintExplanation"`
	Synonyms        string  `json:"synonyms"`
	ImageURL        *string `json:"imageUrl,omitempty"`
	Position        int     `json:"position"`
	Delete          bool    `json:"delete"`
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
// Quiz Question domain (Phase 10)
// ---------------------------------------------------------------------------

// QuizQuestionType is the set of valid quiz question types.
type QuizQuestionType string

const (
	QTypeMultipleChoice QuizQuestionType = "multiple_choice"
	QTypeTrueFalse      QuizQuestionType = "true_false"
	QTypeWritten        QuizQuestionType = "written"
	QTypeParagraph      QuizQuestionType = "paragraph"
	QTypeSorting        QuizQuestionType = "sorting"
)

// QuizQuestion represents a single quiz question within a study set.
type QuizQuestion struct {
	ID                int64               `json:"id"`
	StudySetID        int64               `json:"studySetId"`
	Position          int                 `json:"position"`
	QuestionText      string              `json:"questionText"`
	QuestionType      string              `json:"questionType"`
	CorrectAnswer     *string             `json:"correctAnswer,omitempty"`
	TimeInSeconds     *int                `json:"timeInSeconds,omitempty"`
	AudioURL          *string             `json:"audioUrl,omitempty"`
	AnswerExplanation *string             `json:"answerExplanation,omitempty"`
	ParagraphText     *string             `json:"paragraphText,omitempty"`
	SubQuestions      json.RawMessage     `json:"subQuestions,omitempty"`
	Tags              []string            `json:"tags,omitempty"`
	Options           []QuizQuestionOption `json:"options,omitempty"`
}

// QuizQuestionOption represents an answer option for a quiz question.
type QuizQuestionOption struct {
	ID        int64  `json:"id"`
	QuestionID int64 `json:"questionId"`
	Text      string `json:"text"`
	Position  int    `json:"position"`
	IsCorrect bool   `json:"isCorrect"`
}

// CreateQuizQuestionInput is the payload for creating a quiz question.
type CreateQuizQuestionInput struct {
	Position          int                    `json:"position"`
	QuestionText      string                 `json:"questionText"`
	QuestionType      string                 `json:"questionType"`
	CorrectAnswer     *string                `json:"correctAnswer,omitempty"`
	TimeInSeconds     *int                   `json:"timeInSeconds,omitempty"`
	AudioURL          *string                `json:"audioUrl,omitempty"`
	AnswerExplanation *string                `json:"answerExplanation,omitempty"`
	ParagraphText     *string                `json:"paragraphText,omitempty"`
	SubQuestions      json.RawMessage        `json:"subQuestions,omitempty"`
	Tags              []string               `json:"tags,omitempty"`
	Options           []CreateOptionInput    `json:"options,omitempty"`
}

// CreateOptionInput is the payload for a quiz question option.
type CreateOptionInput struct {
	Text      string `json:"text"`
	Position  int    `json:"position"`
	IsCorrect bool   `json:"isCorrect"`
}

// BulkSaveQuizQuestionsInput replaces all quiz questions for a study set.
type BulkSaveQuizQuestionsInput struct {
	Questions []CreateQuizQuestionInput `json:"questions"`
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

// ImportFlashcardRow represents one row from an Excel import.
type ImportFlashcardRow struct {
	Row             int    `json:"row"`
	Term            string `json:"term"`
	Definition      string `json:"definition"`
	ExampleSentence string `json:"exampleSentence,omitempty"`
	HintExplanation string `json:"hintExplanation,omitempty"`
	Synonyms        string `json:"synonyms,omitempty"`
	ImageURL        string `json:"imageUrl,omitempty"`
}

// ImportQuizRow represents one row from a quiz Excel import.
type ImportQuizRow struct {
	Row                int    `json:"row"`
	Question           string `json:"question"`
	Type               string `json:"type"`
	OptionA            string `json:"optionA,omitempty"`
	OptionB            string `json:"optionB,omitempty"`
	OptionC            string `json:"optionC,omitempty"`
	OptionD            string `json:"optionD,omitempty"`
	CorrectAnswer      string `json:"correctAnswer"`
	TimeSeconds        int    `json:"timeSeconds,omitempty"`
	AudioURL           string `json:"audioUrl,omitempty"`
	AnswerExplanation  string `json:"answerExplanation,omitempty"`
}

// ImportError describes a validation error in a specific row/field.
type ImportError struct {
	Row    int    `json:"row"`
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

// ImportFlashcardResult summarises a flashcard import operation.
type ImportFlashcardResult struct {
	Imported int           `json:"imported"`
	Errors   []ImportError `json:"errors"`
}

// ImportQuizResult summarises a quiz import operation.
type ImportQuizResult struct {
	Imported int           `json:"imported"`
	Errors   []ImportError `json:"errors"`
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
