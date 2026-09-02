package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/http/middleware"
	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/service"
)

type PurchaseHandler struct {
	purchaseSvc *service.PurchaseService
}

func NewPurchaseHandler(purchaseSvc *service.PurchaseService) *PurchaseHandler {
	return &PurchaseHandler{purchaseSvc: purchaseSvc}
}

func (h *PurchaseHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && r.URL.Path == "/v1/entitlements/purchase" {
		h.purchase(w, r)
		return
	}
	WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
}

// POST /v1/entitlements/purchase
func (h *PurchaseHandler) purchase(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req model.PurchaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.StudySetID <= 0 {
		WriteError(w, http.StatusBadRequest, "study_set_id is required")
		return
	}

	resp, err := h.purchaseSvc.PurchaseStudySet(r.Context(), userID, req.StudySetID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInsufficientBalance):
			WriteError(w, http.StatusPaymentRequired, "insufficient balance")
		case errors.Is(err, service.ErrAlreadyOwned):
			WriteRequestError(w, r, http.StatusBadRequest, "already owned this study set", nil)
		case errors.Is(err, service.ErrIsOwner):
			WriteRequestError(w, r, http.StatusBadRequest, "you are the owner of this study set", nil)
		case errors.Is(err, service.ErrFreeSet):
			WriteRequestError(w, r, http.StatusBadRequest, "this study set is free", nil)
		default:
			WriteError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}
	WriteJSON(w, http.StatusOK, resp)
}
