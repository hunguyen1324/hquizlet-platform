package handler

import (
	"net/http"
	"strconv"

	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/http/middleware"
	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/repository"
	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/service"
)

type EntitlementHandler struct {
	accessSvc       *service.AccessService
	entitlementRepo *repository.EntitlementRepo
}

func NewEntitlementHandler(accessSvc *service.AccessService, entitlementRepo *repository.EntitlementRepo) *EntitlementHandler {
	return &EntitlementHandler{accessSvc: accessSvc, entitlementRepo: entitlementRepo}
}

func (h *EntitlementHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/v1/entitlements/check":
		h.checkAccess(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/internal/payment/entitlements/check":
		h.checkAccess(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/v1/entitlements":
		h.listEntitlements(w, r)
	default:
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// GET /v1/entitlements/check?study_set_id=xxx  or
// GET /internal/payment/entitlements/check?study_set_id=xxx
func (h *EntitlementHandler) checkAccess(w http.ResponseWriter, r *http.Request) {
	studySetIDStr := r.URL.Query().Get("study_set_id")
	studySetID, err := strconv.ParseInt(studySetIDStr, 10, 64)
	if err != nil || studySetID <= 0 {
		WriteError(w, http.StatusBadRequest, "study_set_id query parameter is required")
		return
	}

	userID := middleware.GetUserID(r.Context())

	info, err := h.accessSvc.GetAccessInfo(r.Context(), userID, studySetID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	WriteJSON(w, http.StatusOK, info)
}

// GET /v1/entitlements
func (h *EntitlementHandler) listEntitlements(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	entitlements, err := h.entitlementRepo.ListByUser(r.Context(), userID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if entitlements == nil {
		entitlements = []model.Entitlement{}
	}
	WriteJSON(w, http.StatusOK, map[string]any{"items": entitlements})
}
