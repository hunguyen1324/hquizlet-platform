package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/repository"
)

// AccessService checks study set access (ownership, entitlement, pricing).
type AccessService struct {
	db              *sql.DB
	entitlementRepo *repository.EntitlementRepo
	priceRepo       *repository.PriceRepo
	studyServiceURL string
}

func NewAccessService(db *sql.DB, entitlementRepo *repository.EntitlementRepo, priceRepo *repository.PriceRepo, studyServiceURL string) *AccessService {
	return &AccessService{
		db:              db,
		entitlementRepo: entitlementRepo,
		priceRepo:       priceRepo,
		studyServiceURL: studyServiceURL,
	}
}

// GetAccessInfo returns full access info for a study set from the perspective of a user.
func (s *AccessService) GetAccessInfo(ctx context.Context, userID, studySetID int64) (*model.StudySetAccessInfo, error) {
	// 1. Get price (no row = free)
	price, err := s.priceRepo.GetPrice(ctx, studySetID)
	if err != nil {
		return nil, fmt.Errorf("get price: %w", err)
	}

	pricingType := "free"
	priceVnd := 0
	if price != nil {
		pricingType = price.PricingType
		priceVnd = price.PriceVnd
	}

	// 2. Check ownership via Study service internal API
	isOwner, err := s.checkOwnership(ctx, userID, studySetID)
	if err != nil {
		// If we can't reach study service, assume not owner (degrade gracefully)
		isOwner = false
	}

	// 3. Check entitlement
	ent, err := s.entitlementRepo.GetEntitlement(ctx, userID, studySetID)
	if err != nil {
		return nil, fmt.Errorf("check entitlement: %w", err)
	}

	hasAccess := true
	requiresPurchase := false
	var grantedVia *string

	if pricingType == "one_time" && priceVnd > 0 {
		if isOwner {
			hasAccess = true
		} else if ent != nil {
			hasAccess = true
			grantedVia = &ent.GrantedVia
		} else {
			hasAccess = false
			requiresPurchase = true
		}
	}

	return &model.StudySetAccessInfo{
		PricingType:      pricingType,
		PriceVnd:         priceVnd,
		HasAccess:        hasAccess,
		RequiresPurchase: requiresPurchase,
		IsOwner:          isOwner,
		GrantedVia:       grantedVia,
	}, nil
}

// CheckAccess is the quick version for the study service to call.
func (s *AccessService) CheckAccess(ctx context.Context, userID, studySetID int64) (bool, error) {
	info, err := s.GetAccessInfo(ctx, userID, studySetID)
	if err != nil {
		return false, err
	}
	return info.HasAccess, nil
}

// checkOwnership calls Study service internal API to check if user owns the study set.
func (s *AccessService) checkOwnership(ctx context.Context, userID, studySetID int64) (bool, error) {
	url := strings.TrimRight(s.studyServiceURL, "/") + fmt.Sprintf("/internal/study-sets/%d", studySetID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("X-User-ID", fmt.Sprintf("%d", userID))

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("study service returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	// Parse the study set info to get owner_user_id
	var studySetInfo struct {
		UserID int64 `json:"userId"`
	}
	if err := json.Unmarshal(body, &studySetInfo); err != nil {
		return false, err
	}

	return studySetInfo.UserID == userID, nil
}
