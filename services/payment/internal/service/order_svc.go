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
)

type OrderService struct {
	orderRepo *repository.OrderRepo
}

func NewOrderService(orderRepo *repository.OrderRepo) *OrderService {
	return &OrderService{orderRepo: orderRepo}
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

	// Build QR URL
	qrURL := sepay.BuildVietQrURL(
		bankAcct.AccountNumber,
		bankAcct.BankBin,
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
		BankAccountNumber: bankAcct.AccountNumber,
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

	return &model.DepositOrderStatusResponse{
		OrderID:   order.ID,
		Status:    order.Status,
		AmountVnd: order.AmountVnd,
		CreatedAt: order.CreatedAt.Format("2006-01-02T15:04:05Z"),
		QRCodeURL: order.QRCodeURL,
	}, nil
}
