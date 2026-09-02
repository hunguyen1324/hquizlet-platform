package repository

import (
	"context"
	"database/sql"

	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/model"
)

type WalletRepo struct {
	db *sql.DB
}

func NewWalletRepo(db *sql.DB) *WalletRepo {
	return &WalletRepo{db: db}
}

// GetBalance computes balance = SUM(credit) - SUM(debit) from the ledger.
func (r *WalletRepo) GetBalance(ctx context.Context, userID int64) (int, error) {
	var balance sql.NullInt64
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(CASE WHEN direction = 'credit' THEN amount_vnd ELSE 0 END)
		       - SUM(CASE WHEN direction = 'debit' THEN amount_vnd ELSE 0 END), 0)
		 FROM wallet_transaction WHERE user_id = $1`, userID,
	).Scan(&balance)
	if err != nil {
		return 0, err
	}
	return int(balance.Int64), nil
}

// ListTransactions returns paginated transactions for a user.
func (r *WalletRepo) ListTransactions(ctx context.Context, userID int64, limit, offset int) ([]model.WalletTransactionListItem, int, error) {
	// Count total
	var total int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM wallet_transaction WHERE user_id = $1`, userID,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT id, type, direction, amount_vnd, COALESCE(note,''), created_at
		 FROM wallet_transaction WHERE user_id = $1
		 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []model.WalletTransactionListItem
	for rows.Next() {
		var it model.WalletTransactionListItem
		var createdAt sql.NullTime
		if err := rows.Scan(&it.ID, &it.Type, &it.Direction, &it.AmountVnd, &it.Note, &createdAt); err != nil {
			return nil, 0, err
		}
		if createdAt.Valid {
			it.CreatedAt = createdAt.Time.Format("2006-01-02T15:04:05Z")
		}
		it.Label = txLabel(it.Type, it.Direction)
		items = append(items, it)
	}
	return items, total, nil
}

// InsertTransaction inserts a wallet transaction. Used for credit (deposit), debit (purchase), refund, adjustment.
func (r *WalletRepo) InsertTransaction(ctx context.Context, tx *sql.Tx, userID int64, txType, direction string, amountVnd int, refID, note string) (int64, error) {
	var id int64
	var err error
	if tx != nil {
		err = tx.QueryRowContext(ctx,
			`INSERT INTO wallet_transaction (user_id, type, direction, amount_vnd, ref_id, note)
			 VALUES ($1, $2, $3, $4, NULLIF($5,''), NULLIF($6,''))
			 RETURNING id`,
			userID, txType, direction, amountVnd, refID, note,
		).Scan(&id)
	} else {
		err = r.db.QueryRowContext(ctx,
			`INSERT INTO wallet_transaction (user_id, type, direction, amount_vnd, ref_id, note)
			 VALUES ($1, $2, $3, $4, NULLIF($5,''), NULLIF($6,''))
			 RETURNING id`,
			userID, txType, direction, amountVnd, refID, note,
		).Scan(&id)
	}
	return id, err
}

// AdminListAllTransactions lists all transactions across all users (admin only).
func (r *WalletRepo) AdminListAllTransactions(ctx context.Context, limit, offset int) ([]model.WalletTransactionListItem, int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM wallet_transaction`,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT id, type, direction, amount_vnd, COALESCE(note,''), created_at
		 FROM wallet_transaction
		 ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []model.WalletTransactionListItem
	for rows.Next() {
		var it model.WalletTransactionListItem
		var createdAt sql.NullTime
		if err := rows.Scan(&it.ID, &it.Type, &it.Direction, &it.AmountVnd, &it.Note, &createdAt); err != nil {
			return nil, 0, err
		}
		if createdAt.Valid {
			it.CreatedAt = createdAt.Time.Format("2006-01-02T15:04:05Z")
		}
		it.Label = txLabel(it.Type, it.Direction)
		items = append(items, it)
	}
	return items, total, nil
}

func txLabel(txType, direction string) string {
	switch txType {
	case "deposit":
		return "Nạp tiền"
	case "purchase":
		return "Mua study set"
	case "refund":
		return "Hoàn tiền"
	case "adjustment":
		return "Điều chỉnh"
	default:
		return txType
	}
}
