package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// VerifiedIdentity is the response from Auth service internal verify endpoint.
type VerifiedIdentity struct {
	Authenticated bool   `json:"authenticated"`
	UserID        int64  `json:"userId"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	Role          string `json:"role"`
}

// AuthClient is the internal HTTP client for calling Auth service.
type AuthClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewAuthClient creates a new Auth internal HTTP client.
func NewAuthClient(baseURL string) *AuthClient {
	return &AuthClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 3 * time.Second},
	}
}

// VerifyToken verifies a bearer token with the Auth service.
func (c *AuthClient) VerifyToken(ctx context.Context, token string) (*VerifiedIdentity, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/internal/auth/verify", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auth service unavailable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("unauthorized")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("auth service returned %d", resp.StatusCode)
	}

	var identity VerifiedIdentity
	if err := json.NewDecoder(resp.Body).Decode(&identity); err != nil {
		return nil, err
	}
	return &identity, nil
}
