package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/repository"
)

// NewLearnerAccess validates access without returning or charging for card content.
// Protected quizzes record scores on the server and cannot use this write API.
func NewLearnerAccess(sets repository.StudySets, paymentURL string) func(context.Context, int64, int64) error {
	client := &http.Client{Timeout: 3 * time.Second}
	return func(ctx context.Context, setID, uid int64) error {
		set, err := sets.Get(ctx, setID)
		if err != nil {
			return err
		}
		if uid <= 0 || set.Visibility != "public" || set.ContentType == "quiz" {
			return ErrForbidden
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/internal/payment/entitlements/check?study_set_id=%d", strings.TrimRight(paymentURL, "/"), setID), nil)
		if err != nil {
			return err
		}
		req.Header.Set("X-User-ID", fmt.Sprint(uid))
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("access service unavailable")
		}
		var access struct {
			HasAccess bool `json:"hasAccess"`
		}
		if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&access); err != nil {
			return err
		}
		if !access.HasAccess {
			return ErrForbidden
		}
		return nil
	}
}
