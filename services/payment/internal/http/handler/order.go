package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/http/middleware"
	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/service"
)

type OrderHandler struct {
	orderSvc *service.OrderService
}

func NewOrderHandler(orderSvc *service.OrderService) *OrderHandler {
	return &OrderHandler{orderSvc: orderSvc}
}

func (h *OrderHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && r.URL.Path == "/v1/payments/orders" {
		h.createOrder(w, r)
		return
	}
	if r.Method == http.MethodGet {
		parts := PathParts(r.URL.Path, "/v1/payments/orders/")
		if len(parts) > 0 {
			h.getOrderStatus(w, r)
			return
		}
	}
	WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
}

// POST /v1/payments/orders
func (h *OrderHandler) createOrder(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req model.CreateDepositOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	resp, err := h.orderSvc.CreateOrder(r.Context(), userID, req.AmountVnd)
	if err != nil {
		switch err {
		case service.ErrInvalidAmount:
			WriteError(w, http.StatusBadRequest, err.Error())
		case service.ErrRateLimitExceeded:
			WriteRequestError(w, r, http.StatusTooManyRequests, "too many pending orders, please wait", nil)
		default:
			WriteError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}
	WriteJSON(w, http.StatusCreated, resp)
}

// GET /v1/payments/orders/{id}
func (h *OrderHandler) getOrderStatus(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	parts := PathParts(r.URL.Path, "/v1/payments/orders/")
	if len(parts) == 0 {
		WriteError(w, http.StatusBadRequest, "order id required")
		return
	}
	orderID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid order id")
		return
	}

	resp, err := h.orderSvc.GetOrderStatus(r.Context(), orderID, userID)
	if err != nil {
		switch err {
		case service.ErrOrderNotFound:
			WriteError(w, http.StatusNotFound, "order not found")
		default:
			WriteError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}
	WriteJSON(w, http.StatusOK, resp)
}
