package migration

import "database/sql"

// Run applies all payment service migrations idempotently.
func Run(db *sql.DB) error {
	for _, stmt := range migrations {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

var migrations = []string{
	// 001 – wallet_transaction
	`CREATE TABLE IF NOT EXISTS wallet_transaction (
		id           BIGSERIAL    PRIMARY KEY,
		user_id      BIGINT       NOT NULL,
		type         TEXT         NOT NULL CHECK (type IN ('deposit','purchase','refund','adjustment')),
		amount_vnd   INTEGER      NOT NULL,
		direction    TEXT         NOT NULL CHECK (direction IN ('credit','debit')),
		ref_id       TEXT,
		note         TEXT,
		created_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
	)`,
	`CREATE INDEX IF NOT EXISTS idx_wt_user_created
		ON wallet_transaction (user_id, created_at DESC)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uq_wt_deposit_ref
		ON wallet_transaction (ref_id)
		WHERE type = 'deposit'`,

	// 002 – payment_order
	`CREATE TABLE IF NOT EXISTS payment_order (
		id                     BIGSERIAL   PRIMARY KEY,
		user_id                BIGINT      NOT NULL,
		sepay_order_code       TEXT        NOT NULL UNIQUE,
		amount_vnd             INTEGER     NOT NULL,
		status                 TEXT        NOT NULL DEFAULT 'PENDING'
		                               CHECK (status IN ('PENDING','PAID','CANCELLED','EXPIRED')),
		qr_code_url            TEXT,
		expired_at             TIMESTAMPTZ,
		webhook_received_at    TIMESTAMPTZ,
		created_at             TIMESTAMPTZ NOT NULL DEFAULT now()
	)`,
	`CREATE INDEX IF NOT EXISTS idx_po_status ON payment_order (status)`,
	`CREATE INDEX IF NOT EXISTS idx_po_user   ON payment_order (user_id)`,

	// 003 – study_set_price
	`CREATE TABLE IF NOT EXISTS study_set_price (
		study_set_id  BIGINT      PRIMARY KEY,
		pricing_type  TEXT        NOT NULL DEFAULT 'free'
		                          CHECK (pricing_type IN ('free','one_time')),
		price_vnd     INTEGER     NOT NULL DEFAULT 0,
		updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
	)`,

	// 004 – entitlement
	`CREATE TABLE IF NOT EXISTS entitlement (
		id            BIGSERIAL   PRIMARY KEY,
		user_id       BIGINT      NOT NULL,
		study_set_id  BIGINT      NOT NULL,
		granted_via   TEXT        NOT NULL
		                          CHECK (granted_via IN ('purchase','free','admin_grant')),
		tx_id         BIGINT      REFERENCES wallet_transaction(id),
		expires_at    TIMESTAMPTZ,
		created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
		UNIQUE (user_id, study_set_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ent_user ON entitlement (user_id)`,
}
