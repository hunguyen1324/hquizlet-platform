// Package studyclient provides an HTTP client for the Quiz service to call
// the Study service's internal API. P4-INT-01.
//
// The client fetches flashcards by study set ID with ownership verification.
// It follows the same patterns as the auth verify client in the Gateway.
package studyclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// Flashcard mirrors the Study service model.Flashcard JSON shape.
type Flashcard struct {
	ID         int64  `json:"id"`
	StudySetID int64  `json:"studySetId"`
	Term       string `json:"term"`
	Definition string `json:"definition"`
	Starred    bool   `json:"starred"`
	Position   int    `json:"position"`
}

// StudySetWithCards is the response from GET /internal/study-sets/{id}/flashcards.
type StudySetWithCards struct {
	ID          int64       `json:"id"`
	UserID      int64       `json:"userId"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Flashcards  []Flashcard `json:"flashcards"`
}

// ErrorEnvelope matches the standard platform error shape.
type ErrorEnvelope struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId"`
}

// Client calls the Study service's internal API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New creates a Study service client with a 5-second timeout.
func New(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// GetFlashcards fetches all flashcards for a study set.
// The userID is sent via X-User-ID header for ownership verification.
// Returns ErrForbidden if the user doesn't own the set,
// ErrNotFound if the set doesn't exist, or ErrUpstream for transport errors.
func (c *Client) GetFlashcards(ctx context.Context, studySetID, userID int64) (*StudySetWithCards, error) {
	url := fmt.Sprintf("%s/internal/study-sets/%d/flashcards", c.baseURL, studySetID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("X-User-ID", strconv.FormatInt(userID, 10))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &UpstreamError{Op: "GetFlashcards", Err: err}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &UpstreamError{Op: "GetFlashcards", Err: err}
	}

	switch resp.StatusCode {
	case http.StatusOK:
		var studySet StudySetWithCards
		if err := json.Unmarshal(body, &studySet); err != nil {
			return nil, &UpstreamError{Op: "GetFlashcards", Err: fmt.Errorf("decode response: %w", err)}
		}
		return &studySet, nil

	case http.StatusForbidden:
		return nil, ErrForbidden

	case http.StatusNotFound:
		return nil, ErrNotFound

	default:
		var errResp ErrorEnvelope
		_ = json.Unmarshal(body, &errResp)
		return nil, &UpstreamError{
			Op:  "GetFlashcards",
			Err: fmt.Errorf("study service returned %d: %s", resp.StatusCode, errResp.Message),
		}
	}
}

// Sentinel errors for typed error handling.
var (
	ErrForbidden = &ServiceError{Code: "FORBIDDEN", Message: "study set not owned by caller"}
	ErrNotFound  = &ServiceError{Code: "NOT_FOUND", Message: "study set not found"}
)

// ServiceError represents a typed error from the Study service.
type ServiceError struct {
	Code    string
	Message string
}

func (e *ServiceError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// UpstreamError wraps transport-level errors from the Study service.
type UpstreamError struct {
	Op  string
	Err error
}

func (e *UpstreamError) Error() string {
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

func (e *UpstreamError) Unwrap() error {
	return e.Err
}
