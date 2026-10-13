// Package client provides HTTP clients for internal service-to-service calls.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hunguyen1324/hquizlet-platform/packages/authz"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
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
	ID         int64  `json:"id"`
	UserID     int64  `json:"userId"`
	StudySetID int64  `json:"studySetId"`
	Mode       string `json:"mode"`
	Score      int    `json:"score"`
	Total      int    `json:"total"`
	CreatedAt  string `json:"createdAt"`
}

// StudyProgressResponse is the response from the Study progress internal API.
type StudyProgressResponse struct {
	Items []StudyProgressItem `json:"items"`
}

// StudyClient is the internal HTTP client for calling Study service.
type StudyClient struct {
	baseURL       string
	internalToken string
	httpClient    *http.Client
}

// NewStudyClient creates a new Study internal HTTP client.
func NewStudyClient(baseURL, internalToken string) *StudyClient {
	return &StudyClient{
		baseURL:       baseURL,
		internalToken: internalToken,
		httpClient:    &http.Client{Timeout: 5 * time.Second},
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

// StudyProgressSummaryRow is one (user, study set) aggregate from the class dashboard API.
type StudyProgressSummaryRow struct {
	UserID         int64   `json:"userId"`
	StudySetID     int64   `json:"studySetId"`
	Sessions       int     `json:"sessions"`
	BestScore      int     `json:"bestScore"`
	AvgScorePct    int     `json:"avgScorePct"`
	Completed      bool    `json:"completed"`
	LastActivityAt *string `json:"lastActivityAt"`
}

// GetProgressSummary fetches batched progress aggregates for the class dashboard.
// Called with the shared internal token; never from a browser request.
func (c *StudyClient) GetProgressSummary(ctx context.Context, studySetIDs, userIDs []int64) ([]StudyProgressSummaryRow, error) {
	if len(studySetIDs) == 0 || len(userIDs) == 0 {
		return nil, nil
	}
	url := fmt.Sprintf("%s/internal/progress/summary?studySetIds=%s&userIds=%s",
		c.baseURL, joinIDs(studySetIDs), joinIDs(userIDs))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	authz.SignServiceRequest(req, os.Getenv("INTERNAL_SIGNING_KEY"))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("study service unavailable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("study progress summary returned %d: %s", resp.StatusCode, string(body))
	}
	var out struct {
		Items []StudyProgressSummaryRow `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Items, nil
}

func joinIDs(ids []int64) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, strconv.FormatInt(id, 10))
	}
	return strings.Join(parts, ",")
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
