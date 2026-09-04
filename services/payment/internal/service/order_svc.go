package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/repository"
	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/sepay"
)

const (
	maxPendingOrders   = 5
	rateLimitWindowMin = 10
	minDepositVND      = 10000
	maxDepositVND      = 50000000
	orderTTLMinutes    = 30
)

var (
	ErrRateLimitExceeded = errors.New("too many pending orders")
	ErrInvalidAmount     = errors.New("amount must be between 10,000 and 50,000,000 VND")
	ErrOrderNotFound     = errors.New("order not found")
	ErrOrderNotPending   = errors.New("order is not pending")
)

type OrderService struct {
	orderRepo  *repository.OrderRepo
	webhookSvc *WebhookService
}

func NewOrderService(orderRepo *repository.OrderRepo, webhookSvc *WebhookService) *OrderService {
	return &OrderService{orderRepo: orderRepo, webhookSvc: webhookSvc}
}

// CreateOrder creates a new deposit order with rate limiting.
func (s *OrderService) CreateOrder(ctx context.Context, userID int64, amountVnd int) (*model.CreateDepositOrderResponse, error) {
	if amountVnd < minDepositVND || amountVnd > maxDepositVND {
		return nil, ErrInvalidAmount
	}

	// Rate limit: max 5 pending orders per 10 min
	pending, err := s.orderRepo.CountPendingByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("count pending orders: %w", err)
	}
	if pending >= maxPendingOrders {
		return nil, ErrRateLimitExceeded
	}

	// Generate unique order code
	orderCode := sepay.GenerateOrderCode()

	// Get bank account info from SePay
	bankAcct, err := sepay.GetBankAccount()
	if err != nil {
		log.Printf("[payment] WARNING: failed to get bank account from SePay: %v (using placeholder)", err)
		bankAcct = &sepay.BankAccount{
			AccountNumber:     "0000000000",
			AccountHolderName: "HQUIZLET",
			BankShortName:     "BIDV",
			BankBin:           "41540511",
		}
	}

	accountForQR := bankAcct.AccountNumber
	if bankAcct.VA != "" {
		accountForQR = bankAcct.VA
	}
	if cfg := sepay.GetConfig(); cfg.VAAccount != "" {
		accountForQR = cfg.VAAccount
	}
	bankCode := bankAcct.BankShortName
	if bankCode == "" {
		bankCode = bankAcct.BankBin
	}

	// Build QR URL
	qrURL := sepay.BuildVietQrURL(
		accountForQR,
		bankCode,
		bankAcct.AccountHolderName,
		amountVnd,
		orderCode,
	)

	expiredAt := time.Now().UTC().Add(orderTTLMinutes * time.Minute)

	order := &model.PaymentOrder{
		UserID:         userID,
		SepayOrderCode: orderCode,
		AmountVnd:      amountVnd,
		Status:         "PENDING",
		QRCodeURL:      qrURL,
		ExpiredAt:      &expiredAt,
	}

	id, err := s.orderRepo.CreateOrder(ctx, order)
	if err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}
	order.ID = id

	return &model.CreateDepositOrderResponse{
		OrderID:           id,
		OrderCode:         orderCode,
		BankAccountNumber: accountForQR,
		BankAccountHolder: bankAcct.AccountHolderName,
		BankName:          bankAcct.BankShortName,
		AmountVnd:         amountVnd,
		QRCodeURL:         qrURL,
	}, nil
}

// GetOrderStatus returns the status of a deposit order.
func (s *OrderService) GetOrderStatus(ctx context.Context, orderID, userID int64) (*model.DepositOrderStatusResponse, error) {
	order, err := s.orderRepo.GetOrderByID(ctx, orderID)
	if err != nil {
		return nil, ErrOrderNotFound
	}
	if order.UserID != userID {
		return nil, ErrOrderNotFound
	}
	if order.Status == "PENDING" {
		s.reconcilePendingOrder(ctx, order)
		if refreshed, err := s.orderRepo.GetOrderByID(ctx, orderID); err == nil {
			order = refreshed
		}
	}

	return &model.DepositOrderStatusResponse{
		OrderID:   order.ID,
		Status:    order.Status,
		AmountVnd: order.AmountVnd,
		CreatedAt: order.CreatedAt.Format("2006-01-02T15:04:05Z"),
		QRCodeURL: order.QRCodeURL,
	}, nil
}

// ListPendingOrders returns pending deposit orders that the user can cancel.
func (s *OrderService) ListPendingOrders(ctx context.Context, userID int64) ([]model.PendingDepositOrderResponse, error) {
	orders, err := s.orderRepo.ListPendingByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list pending orders: %w", err)
	}
	items := make([]model.PendingDepositOrderResponse, 0, len(orders))
	for _, order := range orders {
		expiredAt := ""
		if order.ExpiredAt != nil {
			expiredAt = order.ExpiredAt.Format("2006-01-02T15:04:05Z")
		}
		items = append(items, model.PendingDepositOrderResponse{
			OrderID:   order.ID,
			OrderCode: order.SepayOrderCode,
			AmountVnd: order.AmountVnd,
			Status:    order.Status,
			CreatedAt: order.CreatedAt.Format("2006-01-02T15:04:05Z"),
			ExpiredAt: expiredAt,
		})
	}
	return items, nil
}

// CancelPendingOrder cancels one owned pending deposit order.
func (s *OrderService) CancelPendingOrder(ctx context.Context, orderID, userID int64) error {
	order, err := s.orderRepo.GetOrderByID(ctx, orderID)
	if err != nil || order.UserID != userID {
		return ErrOrderNotFound
	}
	if order.Status != "PENDING" {
		return ErrOrderNotPending
	}
	ok, err := s.orderRepo.CancelPendingByID(ctx, orderID, userID)
	if err != nil {
		return fmt.Errorf("cancel pending order: %w", err)
	}
	if !ok {
		return ErrOrderNotPending
	}
	return nil
}

func (s *OrderService) reconcilePendingOrder(ctx context.Context, order *model.PaymentOrder) {
	if s.webhookSvc == nil {
		return
	}
	tx, err := sepay.FindIncomingTransaction(order.SepayOrderCode, order.AmountVnd)
	if err != nil {
		log.Printf("[payment] reconcile: SePay lookup failed for order %s: %v", order.SepayOrderCode, err)
		return
	}
	if tx == nil {
		return
	}
	refID := tx.ID
	if refID == "" {
		refID = tx.ReferenceNumber
	}
	result := s.webhookSvc.CreditDepositIfPaidRef(ctx, order.SepayOrderCode, tx.AmountIn, refID)
	log.Printf("[payment] reconcile: order=%s sepay_tx=%s result=%s", order.SepayOrderCode, refID, result)
}
