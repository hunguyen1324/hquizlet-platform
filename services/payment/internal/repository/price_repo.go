package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/model"
)

type PriceRepo struct {
	db *sql.DB
}

func NewPriceRepo(db *sql.DB) *PriceRepo {
	return &PriceRepo{db: db}
}

// GetPrice returns the price for a study set. Returns nil if no row (meaning free).
func (r *PriceRepo) GetPrice(ctx context.Context, studySetID int64) (*model.StudySetPrice, error) {
	var p model.StudySetPrice
	err := r.db.QueryRowContext(ctx,
		`SELECT study_set_id, pricing_type, price_vnd, updated_at
		 FROM study_set_price WHERE study_set_id = $1`, studySetID,
	).Scan(&p.StudySetID, &p.PricingType, &p.PriceVnd, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// UpsertPrice creates or updates the price for a study set.
func (r *PriceRepo) UpsertPrice(ctx context.Context, studySetID int64, pricingType string, priceVnd int) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO study_set_price (study_set_id, pricing_type, price_vnd, updated_at)
		 VALUES ($1, $2, $3, now())
		 ON CONFLICT (study_set_id) DO UPDATE
		 SET pricing_type = EXCLUDED.pricing_type, price_vnd = EXCLUDED.price_vnd, updated_at = now()`,
		studySetID, pricingType, priceVnd,
	)
	return err
}

// GetBalance is a convenience method on the DB for use in purchase transaction.
func GetBalanceForUser(ctx context.Context, db *sql.DB, userID int64) (int, error) {
	var balance sql.NullInt64
	err := db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(CASE WHEN direction = 'credit' THEN amount_vnd ELSE 0 END)
		       - SUM(CASE WHEN direction = 'debit' THEN amount_vnd ELSE 0 END), 0)
		 FROM wallet_transaction WHERE user_id = $1`, userID,
	).Scan(&balance)
	if err != nil {
		return 0, err
	}
	return int(balance.Int64), nil
}

// DebitWallet inserts a debit transaction. Called within a DB transaction.
func DebitWallet(ctx context.Context, tx *sql.Tx, userID int64, amountVnd int, refID, note string) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx,
		`INSERT INTO wallet_transaction (user_id, type, direction, amount_vnd, ref_id, note)
		 VALUES ($1, 'purchase', 'debit', $2, NULLIF($3,''), NULLIF($4,''))
		 RETURNING id`,
		userID, amountVnd, refID, note,
	).Scan(&id)
	return id, err
}

// Now returns current UTC time (for use in transactions).
func Now() time.Time {
	return time.Now().UTC()
}
