package service

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/repository"
)

type WebhookService struct {
	db         *sql.DB
	orderRepo  *repository.OrderRepo
	walletRepo *repository.WalletRepo
}

func NewWebhookService(db *sql.DB, orderRepo *repository.OrderRepo, walletRepo *repository.WalletRepo) *WebhookService {
	return &WebhookService{db: db, orderRepo: orderRepo, walletRepo: walletRepo}
}

// WebhookResult is the outcome of processing a webhook.
type WebhookResult string

const (
	ResultCredited       WebhookResult = "credited"
	ResultAlreadyProcessed WebhookResult = "already_processed"
	ResultAmountMismatch   WebhookResult = "amount_mismatch"
	ResultOrderNotFound    WebhookResult = "order_not_found"
)

// CreditDepositIfPaid processes an incoming webhook, credits the wallet idempotently.
func (s *WebhookService) CreditDepositIfPaid(ctx context.Context, orderCode string, payloadTransferAmount int, sepayTxID int64) WebhookResult {
	order, err := s.orderRepo.GetOrderByCode(ctx, orderCode)
	if err != nil {
		if err == sql.ErrNoRows {
			return ResultOrderNotFound
		}
		log.Printf("[payment] webhook: error finding order %s: %v", orderCode, err)
		return ResultOrderNotFound
	}

	// Begin transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("[payment] webhook: begin tx error: %v", err)
		return ResultOrderNotFound
	}
	defer tx.Rollback()

	// Lock the order row for update
	var status string
	err = tx.QueryRowContext(ctx,
		`SELECT status FROM payment_order WHERE id = $1 FOR UPDATE`, order.ID,
	).Scan(&status)
	if err != nil {
		log.Printf("[payment] webhook: lock order error: %v", err)
		return ResultOrderNotFound
	}

	if status != "PENDING" {
		return ResultAlreadyProcessed
	}

	if order.AmountVnd != payloadTransferAmount {
		log.Printf("[payment] AMOUNT MISMATCH: order %s expected %d got %d (orderID=%d, userID=%d)",
			orderCode, order.AmountVnd, payloadTransferAmount, order.ID, order.UserID)
		return ResultAmountMismatch
	}

	// Insert wallet transaction (credit) - dedup via partial unique index
	refID := fmt.Sprintf("%d", sepayTxID)
	note := "Nạp tiền qua SePay"
	var txID int64
	err = tx.QueryRowContext(ctx,
		`INSERT INTO wallet_transaction (user_id, type, direction, amount_vnd, ref_id, note)
		 VALUES ($1, 'deposit', 'credit', $2, $3, $4)
		 ON CONFLICT (ref_id) WHERE type = 'deposit' DO NOTHING
		 RETURNING id`,
		order.UserID, order.AmountVnd, refID, note,
	).Scan(&txID)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("[payment] webhook: insert wallet tx error: %v", err)
		return ResultOrderNotFound
	}

	// Update order status to PAID
	now := time.Now().UTC()
	_, err = tx.ExecContext(ctx,
		`UPDATE payment_order SET status = 'PAID', webhook_received_at = $1 WHERE id = $2`,
		now, order.ID,
	)
	if err != nil {
		log.Printf("[payment] webhook: update order status error: %v", err)
		return ResultOrderNotFound
	}

	if err := tx.Commit(); err != nil {
		log.Printf("[payment] webhook: commit error: %v", err)
		return ResultOrderNotFound
	}

	return ResultCredited
}
