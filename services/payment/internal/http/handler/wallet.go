package handler

import (
	"net/http"
	"strconv"

	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/http/middleware"
	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/service"
)

type WalletHandler struct {
	walletSvc *service.WalletService
}

func NewWalletHandler(walletSvc *service.WalletService) *WalletHandler {
	return &WalletHandler{walletSvc: walletSvc}
}

func (h *WalletHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/v1/wallet":
		h.getBalance(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/v1/wallet/transactions":
		h.listTransactions(w, r)
	default:
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *WalletHandler) getBalance(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	balance, err := h.walletSvc.GetBalance(r.Context(), userID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]int{"balance": balance})
}

func (h *WalletHandler) listTransactions(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	q := r.URL.Query()
	limit := intQueryParam(q.Get("limit"), 20)
	offset := intQueryParam(q.Get("offset"), 0)

	result, err := h.walletSvc.GetTransactions(r.Context(), userID, limit, offset)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	WriteJSON(w, http.StatusOK, result)
}

func intQueryParam(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return def
	}
	return n
}
