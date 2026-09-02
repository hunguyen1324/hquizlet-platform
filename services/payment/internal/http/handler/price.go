package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/http/middleware"
	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/repository"
	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/service"
)

type PriceHandler struct {
	priceRepo *repository.PriceRepo
	accessSvc *service.AccessService
}

func NewPriceHandler(priceRepo *repository.PriceRepo, accessSvc *service.AccessService) *PriceHandler {
	return &PriceHandler{priceRepo: priceRepo, accessSvc: accessSvc}
}

func (h *PriceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPut {
		parts := PathParts(r.URL.Path, "/v1/study-sets/")
		if len(parts) >= 2 && parts[1] == "price" {
			h.setPrice(w, r)
			return
		}
	}
	WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
}

// PUT /v1/study-sets/{id}/price
func (h *PriceHandler) setPrice(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	parts := PathParts(r.URL.Path, "/v1/study-sets/")
	if len(parts) == 0 {
		WriteError(w, http.StatusBadRequest, "study set id required")
		return
	}
	studySetID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid study set id")
		return
	}

	// Verify ownership
	accessInfo, err := h.accessSvc.GetAccessInfo(r.Context(), userID, studySetID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if !accessInfo.IsOwner {
		WriteError(w, http.StatusForbidden, "only the owner can set the price")
		return
	}

	var req model.SetStudySetPriceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// Validate pricing type
	if req.PricingType != "free" && req.PricingType != "one_time" {
		WriteError(w, http.StatusBadRequest, "pricing_type must be 'free' or 'one_time'")
		return
	}
	if req.PricingType == "one_time" && req.PriceVnd <= 0 {
		WriteError(w, http.StatusBadRequest, "price_vnd must be > 0 for one_time pricing")
		return
	}

	if err := h.priceRepo.UpsertPrice(r.Context(), studySetID, req.PricingType, req.PriceVnd); err != nil {
		WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"studySetId":  studySetID,
		"pricingType": req.PricingType,
		"priceVnd":    req.PriceVnd,
	})
}
