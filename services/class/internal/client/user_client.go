package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// UserDirectoryEntry is a minimal user record from the auth service internal API.
type UserDirectoryEntry struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

// UserClient resolves user ids to display records via the auth service.
type UserClient struct {
	baseURL       string
	internalToken string
	httpClient    *http.Client
}

func NewUserClient(baseURL, internalToken string) *UserClient {
	return &UserClient{
		baseURL:       baseURL,
		internalToken: internalToken,
		httpClient:    &http.Client{Timeout: 3 * time.Second},
	}
}

// Lookup returns a map of userID -> directory entry. Missing ids are simply absent;
// callers must tolerate unknown users (best-effort display data).
func (c *UserClient) Lookup(ctx context.Context, ids []int64) (map[int64]UserDirectoryEntry, error) {
	if len(ids) == 0 {
		return map[int64]UserDirectoryEntry{}, nil
	}
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, fmt.Sprintf("%d", id))
	}
	url := fmt.Sprintf("%s/internal/users?ids=%s", c.baseURL, strings.Join(parts, ","))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Internal-Token", c.internalToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auth service unavailable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("auth directory returned %d: %s", resp.StatusCode, string(body))
	}
	var out struct {
		Users []UserDirectoryEntry `json:"users"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	byID := make(map[int64]UserDirectoryEntry, len(out.Users))
	for _, u := range out.Users {
		byID[u.ID] = u
	}
	return byID, nil
}
