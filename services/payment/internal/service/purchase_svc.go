package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/repository"
)

var (
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrAlreadyOwned        = errors.New("already owned this study set")
	ErrIsOwner             = errors.New("you are the owner of this study set")
	ErrFreeSet             = errors.New("this study set is free")
	ErrSetNotFound         = errors.New("study set not found")
)

type PurchaseService struct {
	db              *sql.DB
	entitlementRepo *repository.EntitlementRepo
	priceRepo       *repository.PriceRepo
	walletRepo      *repository.WalletRepo
}

func NewPurchaseService(db *sql.DB, entitlementRepo *repository.EntitlementRepo, priceRepo *repository.PriceRepo, walletRepo *repository.WalletRepo) *PurchaseService {
	return &PurchaseService{
		db:              db,
		entitlementRepo: entitlementRepo,
		priceRepo:       priceRepo,
		walletRepo:      walletRepo,
	}
}

// PurchaseStudySet buys a study set using the wallet (atomic debit + entitlement).
func (s *PurchaseService) PurchaseStudySet(ctx context.Context, userID, studySetID int64) (*model.PurchaseResponse, error) {
	// 1. Get price
	price, err := s.priceRepo.GetPrice(ctx, studySetID)
	if err != nil {
		return nil, fmt.Errorf("get price: %w", err)
	}
	if price == nil || price.PricingType == "free" {
		return nil, ErrFreeSet
	}
	if price.PriceVnd <= 0 {
		return nil, ErrFreeSet
	}

	// 2. Check if already owned
	existing, err := s.entitlementRepo.GetEntitlement(ctx, userID, studySetID)
	if err != nil {
		return nil, fmt.Errorf("check entitlement: %w", err)
	}
	if existing != nil {
		return nil, ErrAlreadyOwned
	}

	// 3. Check balance
	balance, err := repository.GetBalanceForUser(ctx, s.db, userID)
	if err != nil {
		return nil, fmt.Errorf("get balance: %w", err)
	}
	if balance < price.PriceVnd {
		return nil, ErrInsufficientBalance
	}

	// 4. Atomic DB transaction: debit + entitlement
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Debit wallet
	note := fmt.Sprintf("Mua study set #%d", studySetID)
	txID, err := repository.DebitWallet(ctx, tx, userID, price.PriceVnd, fmt.Sprintf("set_%d", studySetID), note)
	if err != nil {
		log.Printf("[payment] purchase: debit error: %v", err)
		return nil, fmt.Errorf("debit wallet: %w", err)
	}

	// Grant entitlement
	ent := &model.Entitlement{
		UserID:     userID,
		StudySetID: studySetID,
		GrantedVia: "purchase",
		TxID:       &txID,
	}
	_, err = s.entitlementRepo.InsertEntitlement(ctx, tx, ent)
	if err != nil {
		log.Printf("[payment] purchase: entitlement insert error: %v", err)
		return nil, fmt.Errorf("insert entitlement: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	// Get new balance
	newBalance, err := repository.GetBalanceForUser(ctx, s.db, userID)
	if err != nil {
		newBalance = balance - price.PriceVnd
	}

	return &model.PurchaseResponse{
		Balance:  newBalance,
		PriceVnd: price.PriceVnd,
	}, nil
}
