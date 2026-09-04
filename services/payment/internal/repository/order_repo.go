package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/model"
)

type OrderRepo struct {
	db *sql.DB
}

func NewOrderRepo(db *sql.DB) *OrderRepo {
	return &OrderRepo{db: db}
}

// CreateOrder inserts a new deposit order.
func (r *OrderRepo) CreateOrder(ctx context.Context, o *model.PaymentOrder) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO payment_order (user_id, sepay_order_code, amount_vnd, status, qr_code_url, expired_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id`,
		o.UserID, o.SepayOrderCode, o.AmountVnd, o.Status, o.QRCodeURL, o.ExpiredAt,
	).Scan(&id)
	return id, err
}

// GetOrderByCode returns an order by its SePay order code.
func (r *OrderRepo) GetOrderByCode(ctx context.Context, code string) (*model.PaymentOrder, error) {
	return r.queryOne(ctx,
		`SELECT id, user_id, sepay_order_code, amount_vnd, status, COALESCE(qr_code_url,''),
		        expired_at, webhook_received_at, created_at
		 FROM payment_order WHERE sepay_order_code = $1`, code)
}

// GetOrderByID returns an order by its ID.
func (r *OrderRepo) GetOrderByID(ctx context.Context, id int64) (*model.PaymentOrder, error) {
	return r.queryOne(ctx,
		`SELECT id, user_id, sepay_order_code, amount_vnd, status, COALESCE(qr_code_url,''),
		        expired_at, webhook_received_at, created_at
		 FROM payment_order WHERE id = $1`, id)
}

// UpdateStatus sets the status and optionally the webhook_received_at.
func (r *OrderRepo) UpdateStatus(ctx context.Context, id int64, status string, webhookTime *time.Time) error {
	if webhookTime != nil {
		_, err := r.db.ExecContext(ctx,
			`UPDATE payment_order SET status = $1, webhook_received_at = $2 WHERE id = $3`,
			status, *webhookTime, id)
		return err
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE payment_order SET status = $1 WHERE id = $2`, status, id)
	return err
}

// CountPendingByUser returns how many PENDING orders a user has.
func (r *OrderRepo) CountPendingByUser(ctx context.Context, userID int64) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM payment_order WHERE user_id = $1 AND status = 'PENDING'`, userID,
	).Scan(&count)
	return count, err
}

// ListPendingByUser returns a user's PENDING deposit orders newest first.
func (r *OrderRepo) ListPendingByUser(ctx context.Context, userID int64) ([]model.PaymentOrder, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, sepay_order_code, amount_vnd, status, COALESCE(qr_code_url,''),
		        expired_at, webhook_received_at, created_at
		 FROM payment_order
		 WHERE user_id = $1 AND status = 'PENDING'
		 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []model.PaymentOrder{}
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *o)
	}
	return items, rows.Err()
}

// CancelPendingByID marks one owned PENDING order as CANCELLED.
func (r *OrderRepo) CancelPendingByID(ctx context.Context, id, userID int64) (bool, error) {
	result, err := r.db.ExecContext(ctx,
		`UPDATE payment_order
		 SET status = 'CANCELLED'
		 WHERE id = $1 AND user_id = $2 AND status = 'PENDING'`,
		id, userID,
	)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

// ListAllOrders returns all orders (admin only).
func (r *OrderRepo) ListAllOrders(ctx context.Context, limit, offset int) ([]model.PaymentOrder, int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM payment_order`,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, sepay_order_code, amount_vnd, status, COALESCE(qr_code_url,''),
		        expired_at, webhook_received_at, created_at
		 FROM payment_order ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []model.PaymentOrder
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *o)
	}
	return items, total, nil
}

type scannable interface {
	Scan(dest ...any) error
}

func (r *OrderRepo) queryOne(ctx context.Context, query string, args ...any) (*model.PaymentOrder, error) {
	row := r.db.QueryRowContext(ctx, query, args...)
	return scanOrder(row)
}

func scanOrder(row scannable) (*model.PaymentOrder, error) {
	var o model.PaymentOrder
	var expiredAt, webhookAt sql.NullTime
	err := row.Scan(&o.ID, &o.UserID, &o.SepayOrderCode, &o.AmountVnd, &o.Status, &o.QRCodeURL,
		&expiredAt, &webhookAt, &o.CreatedAt)
	if err != nil {
		return nil, err
	}
	if expiredAt.Valid {
		o.ExpiredAt = &expiredAt.Time
	}
	if webhookAt.Valid {
		o.WebhookReceivedAt = &webhookAt.Time
	}
	return &o, nil
}
