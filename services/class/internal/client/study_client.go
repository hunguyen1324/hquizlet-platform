// Package client provides HTTP clients for internal service-to-service calls.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// StudySetInfo is the minimal info returned by Study service internal API.
type StudySetInfo struct {
	ID    int64  `json:"id"`
	UID   int64  `json:"userId"`
	Title string `json:"title"`
}

// StudyProgressItem represents a recent progress record from Study service.
type StudyProgressItem struct {
	ID          int64  `json:"id"`
	UserID      int64  `json:"userId"`
	StudySetID  int64  `json:"studySetId"`
	Mode        string `json:"mode"`
	Score       int    `json:"score"`
	Total       int    `json:"total"`
	CreatedAt   string `json:"createdAt"`
}

// StudyProgressResponse is the response from the Study progress internal API.
type StudyProgressResponse struct {
	Items []StudyProgressItem `json:"items"`
}

// StudyClient is the internal HTTP client for calling Study service.
type StudyClient struct {
	baseURL    string
	internalToken string
	httpClient *http.Client
}

// NewStudyClient creates a new Study internal HTTP client.
func NewStudyClient(baseURL, internalToken string) *StudyClient {
	return &StudyClient{
		baseURL:    baseURL,
		internalToken: internalToken,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// GetStudySet fetches study set info from Study service.
func (c *StudyClient) GetStudySet(ctx context.Context, studySetID int64) (*StudySetInfo, error) {
	url := fmt.Sprintf("%s/internal/study-sets/%d", c.baseURL, studySetID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Internal-Token", c.internalToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("study service unavailable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("study service returned %d: %s", resp.StatusCode, string(body))
	}

	var info StudySetInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

// GetRecentProgress fetches recent study progress from Study service.
func (c *StudyClient) GetRecentProgress(ctx context.Context, userID int64, limit int) ([]StudyProgressItem, error) {
	url := fmt.Sprintf("%s/internal/study/progress/recent?userId=%d&limit=%d", c.baseURL, userID, limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Internal-Token", c.internalToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Study service unavailable — return empty, don't fail
		return nil, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Non-success — return empty, don't fail activity feed
		return nil, nil
	}

	var result StudyProgressResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, nil
	}
	return result.Items, nil
}
