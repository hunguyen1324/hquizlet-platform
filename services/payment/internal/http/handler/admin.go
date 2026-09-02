package handler

import (
	"encoding/json"
	"net/http"

	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/repository"
	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/service"
)

type AdminHandler struct {
	walletSvc *service.WalletService
	orderRepo *repository.OrderRepo
}

func NewAdminHandler(walletSvc *service.WalletService, orderRepo *repository.OrderRepo) *AdminHandler {
	return &AdminHandler{walletSvc: walletSvc, orderRepo: orderRepo}
}

func (h *AdminHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/v1/admin/payments/orders":
		h.listOrders(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/v1/admin/wallet/transactions":
		h.listTransactions(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/admin/wallet/credit":
		h.credit(w, r)
	default:
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// GET /v1/admin/payments/orders
func (h *AdminHandler) listOrders(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := intQueryParam(q.Get("limit"), 20)
	offset := intQueryParam(q.Get("offset"), 0)

	items, total, err := h.orderRepo.ListAllOrders(r.Context(), limit, offset)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if items == nil {
		items = []model.PaymentOrder{}
	}
	WriteJSON(w, http.StatusOK, model.AdminOrderListResponse{Items: items, Total: total})
}

// GET /v1/admin/wallet/transactions
func (h *AdminHandler) listTransactions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := intQueryParam(q.Get("limit"), 20)
	offset := intQueryParam(q.Get("offset"), 0)

	result, err := h.walletSvc.AdminListTransactions(r.Context(), limit, offset)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	WriteJSON(w, http.StatusOK, result)
}

// POST /v1/admin/wallet/credit
func (h *AdminHandler) credit(w http.ResponseWriter, r *http.Request) {
	var req model.AdminCreditRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.UserID <= 0 {
		WriteError(w, http.StatusBadRequest, "user_id is required")
		return
	}
	if req.AmountVnd <= 0 {
		WriteError(w, http.StatusBadRequest, "amount_vnd must be > 0")
		return
	}

	if err := h.walletSvc.AdminCredit(r.Context(), req.UserID, req.AmountVnd, req.Note); err != nil {
		WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"userId": req.UserID,
		"amount": req.AmountVnd,
	})
}
