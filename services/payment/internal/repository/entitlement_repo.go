package repository

import (
	"context"
	"database/sql"

	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/model"
)

type EntitlementRepo struct {
	db *sql.DB
}

func NewEntitlementRepo(db *sql.DB) *EntitlementRepo {
	return &EntitlementRepo{db: db}
}

// GetEntitlement returns the entitlement for a user+study_set, or nil if none exists.
func (r *EntitlementRepo) GetEntitlement(ctx context.Context, userID, studySetID int64) (*model.Entitlement, error) {
	var e model.Entitlement
	var txID sql.NullInt64
	var expiresAt sql.NullTime
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, study_set_id, granted_via, tx_id, expires_at, created_at
		 FROM entitlement WHERE user_id = $1 AND study_set_id = $2`,
		userID, studySetID,
	).Scan(&e.ID, &e.UserID, &e.StudySetID, &e.GrantedVia, &txID, &expiresAt, &e.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if txID.Valid {
		e.TxID = &txID.Int64
	}
	if expiresAt.Valid {
		e.ExpiresAt = &expiresAt.Time
	}
	return &e, nil
}

// InsertEntitlement grants access to a study set.
func (r *EntitlementRepo) InsertEntitlement(ctx context.Context, tx *sql.Tx, e *model.Entitlement) (int64, error) {
	var id int64
	q := `INSERT INTO entitlement (user_id, study_set_id, granted_via, tx_id, expires_at)
	      VALUES ($1, $2, $3, $4, $5)
	      ON CONFLICT (user_id, study_set_id) DO NOTHING
	      RETURNING id`

	var txID sql.NullInt64
	if e.TxID != nil {
		txID = sql.NullInt64{Int64: *e.TxID, Valid: true}
	}

	var err error
	if tx != nil {
		err = tx.QueryRowContext(ctx, q, e.UserID, e.StudySetID, e.GrantedVia, txID, nil).Scan(&id)
	} else {
		err = r.db.QueryRowContext(ctx, q, e.UserID, e.StudySetID, e.GrantedVia, txID, nil).Scan(&id)
	}
	if err == sql.ErrNoRows {
		// ON CONFLICT DO NOTHING returns no row - that's fine, entitlement already exists
		return 0, nil
	}
	return id, err
}

// ListByUser returns all entitlements for a user.
func (r *EntitlementRepo) ListByUser(ctx context.Context, userID int64) ([]model.Entitlement, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, study_set_id, granted_via, tx_id, expires_at, created_at
		 FROM entitlement WHERE user_id = $1 ORDER BY created_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.Entitlement
	for rows.Next() {
		var e model.Entitlement
		var txID sql.NullInt64
		var expiresAt sql.NullTime
		if err := rows.Scan(&e.ID, &e.UserID, &e.StudySetID, &e.GrantedVia, &txID, &expiresAt, &e.CreatedAt); err != nil {
			return nil, err
		}
		if txID.Valid {
			e.TxID = &txID.Int64
		}
		if expiresAt.Valid {
			e.ExpiresAt = &expiresAt.Time
		}
		items = append(items, e)
	}
	return items, nil
}
