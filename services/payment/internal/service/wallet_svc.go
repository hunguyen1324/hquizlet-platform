package service

import (
	"context"

	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/repository"
)

type WalletService struct {
	walletRepo *repository.WalletRepo
}

func NewWalletService(walletRepo *repository.WalletRepo) *WalletService {
	return &WalletService{walletRepo: walletRepo}
}

// GetBalance returns the current wallet balance for a user.
func (s *WalletService) GetBalance(ctx context.Context, userID int64) (int, error) {
	return s.walletRepo.GetBalance(ctx, userID)
}

// GetTransactions returns paginated transactions for a user.
func (s *WalletService) GetTransactions(ctx context.Context, userID int64, limit, offset int) (*model.WalletTransactionListResponse, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	items, total, err := s.walletRepo.ListTransactions(ctx, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.WalletTransactionListItem{}
	}
	return &model.WalletTransactionListResponse{Items: items, Total: total}, nil
}

// AdminCredit manually credits a user's wallet (admin operation).
func (s *WalletService) AdminCredit(ctx context.Context, userID int64, amountVnd int, note string) error {
	if amountVnd <= 0 {
		return nil
	}
	_, err := s.walletRepo.InsertTransaction(ctx, nil, userID, "adjustment", "credit", amountVnd, "", note)
	return err
}

// AdminListOrders returns all orders (admin only).
func (s *WalletService) AdminListOrders(ctx context.Context, limit, offset int) ([]model.PaymentOrder, int, error) {
	return nil, 0, nil // Implemented via orderRepo
}

// AdminListTransactions returns all transactions (admin only).
func (s *WalletService) AdminListTransactions(ctx context.Context, limit, offset int) (*model.AdminTxListResponse, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	items, total, err := s.walletRepo.AdminListAllTransactions(ctx, limit, offset)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.WalletTransactionListItem{}
	}
	return &model.AdminTxListResponse{Items: items, Total: total}, nil
}
