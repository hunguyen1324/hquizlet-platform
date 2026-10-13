package migration

import "database/sql"

// Run applies all auth migrations idempotently.
// Safe to call on a fresh DB or an existing DB from older versions.
// Each statement uses IF NOT EXISTS or DO $$ blocks so repeated runs are harmless.
func Run(db *sql.DB) error {
	for _, stmt := range migrations {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

var migrations = []string{
	// 001 – create users table (fresh DB)
	`CREATE TABLE IF NOT EXISTS users (
		id            BIGSERIAL    PRIMARY KEY,
		name          TEXT         NOT NULL,
		email         TEXT         NOT NULL UNIQUE,
		password_hash TEXT         NOT NULL,
		image         TEXT         NOT NULL DEFAULT '',
		role          TEXT         NOT NULL DEFAULT 'user',
		created_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
	)`,

	// 002 – add image column if DB existed before it was introduced
	`DO $$ BEGIN
		IF NOT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'users' AND column_name = 'image'
		) THEN
			ALTER TABLE users ADD COLUMN image TEXT NOT NULL DEFAULT '';
		END IF;
	END $$`,

	// 003 – add role column if DB existed before it was introduced
	`DO $$ BEGIN
		IF NOT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'users' AND column_name = 'role'
		) THEN
			ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'user';
		END IF;
	END $$`,

	// 004 – create sessions table
	`CREATE TABLE IF NOT EXISTS sessions (
		id         BIGSERIAL    PRIMARY KEY,
		user_id    BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		token_hash TEXT         NOT NULL UNIQUE,
		expires_at TIMESTAMPTZ  NOT NULL,
		created_at TIMESTAMPTZ  NOT NULL DEFAULT now()
	)`,

	// 005 – indexes
	`CREATE INDEX IF NOT EXISTS sessions_token_hash_idx ON sessions(token_hash)`,
	`CREATE INDEX IF NOT EXISTS sessions_user_id_idx    ON sessions(user_id)`,

	// 006 – Phase 3 revocation and account status. Append-only and upgrade-safe.
	`ALTER TABLE users ADD COLUMN IF NOT EXISTS disabled BOOLEAN NOT NULL DEFAULT false`,
	`ALTER TABLE sessions ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ`,

	// 007 – IAM roles, assignments, audit (dashboard plan Phase 1)
	`CREATE TABLE IF NOT EXISTS iam_roles (
		id           BIGSERIAL PRIMARY KEY,
		key          TEXT NOT NULL,
		name         TEXT NOT NULL,
		description  TEXT NOT NULL DEFAULT '',
		kind         TEXT NOT NULL DEFAULT 'managed' CHECK (kind IN ('managed','custom')),
		org_id       BIGINT NULL,
		created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
		UNIQUE (key, org_id)
	)`,
	`CREATE TABLE IF NOT EXISTS iam_assignments (
		id             BIGSERIAL PRIMARY KEY,
		principal_type TEXT   NOT NULL DEFAULT 'user',
		principal_id   BIGINT NOT NULL,
		role_key       TEXT   NOT NULL,
		scope_type     TEXT   NOT NULL CHECK (scope_type IN ('platform','org')),
		scope_id       BIGINT NULL,
		created_by     BIGINT NOT NULL,
		created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
		expires_at     TIMESTAMPTZ NULL,
		UNIQUE (principal_type, principal_id, role_key, scope_type, scope_id)
	)`,
	`CREATE INDEX IF NOT EXISTS iam_assignments_principal_idx ON iam_assignments(principal_type, principal_id)`,
	`CREATE TABLE IF NOT EXISTS audit_log (
		id          BIGSERIAL PRIMARY KEY,
		actor_id    BIGINT NOT NULL,
		action      TEXT   NOT NULL,
		resource    TEXT   NOT NULL,
		result      TEXT   NOT NULL CHECK (result IN ('allow','deny')),
		request_id  TEXT,
		ip          INET,
		detail      JSONB,
		at          TIMESTAMPTZ NOT NULL DEFAULT now()
	)`,
	`CREATE INDEX IF NOT EXISTS audit_log_at_idx ON audit_log(at DESC)`,
	`CREATE INDEX IF NOT EXISTS audit_log_actor_idx ON audit_log(actor_id, at DESC)`,

	// 008 – backfill platform roles from legacy users.role (idempotent)
	`INSERT INTO iam_assignments (principal_type, principal_id, role_key, scope_type, scope_id, created_by)
	 SELECT 'user', id, 'PlatformAdmin', 'platform', NULL, id FROM users u WHERE role = 'admin' AND NOT EXISTS (SELECT 1 FROM iam_assignments a WHERE a.principal_id=u.id AND a.principal_type='user' AND a.scope_type='platform' AND a.role_key='PlatformAdmin')
	 ON CONFLICT (principal_type, principal_id, role_key, scope_type, scope_id) DO NOTHING`,
	`INSERT INTO iam_assignments (principal_type, principal_id, role_key, scope_type, scope_id, created_by)
	 SELECT 'user', id, 'Student', 'platform', NULL, id FROM users u WHERE role <> 'admin' AND NOT EXISTS (SELECT 1 FROM iam_assignments a WHERE a.principal_id=u.id AND a.principal_type='user' AND a.scope_type='platform')
	 ON CONFLICT (principal_type, principal_id, role_key, scope_type, scope_id) DO NOTHING`,

	// 009 – organizations (dashboard plan Phase 5 / ORG-01).
	// iam_assignments.scope_type already allows 'org', so memberships need no new table.
	// The class service owns classes.org_id and adds it in its own migration.
	`CREATE TABLE IF NOT EXISTS organizations (
		id            BIGSERIAL   PRIMARY KEY,
		name          TEXT        NOT NULL,
		slug          TEXT        NOT NULL UNIQUE,
		kind          TEXT        NOT NULL DEFAULT 'school' CHECK (kind IN ('school','company','other')),
		owner_user_id BIGINT      NOT NULL,
		created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
		updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
	)`,
	`CREATE INDEX IF NOT EXISTS organizations_owner_idx ON organizations(owner_user_id)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS iam_assignments_org_unique_idx
	 ON iam_assignments(principal_type, principal_id, role_key, scope_id)
	 WHERE scope_type = 'org'`,
	// Enforce uniqueness for NULL platform scopes on upgrades, preserving the newest non-expiring grant.
	`DELETE FROM iam_roles WHERE id IN (SELECT id FROM (SELECT id,row_number() OVER(PARTITION BY key,org_id ORDER BY id) n FROM iam_roles) d WHERE n>1)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS iam_roles_nulls_unique ON iam_roles(key,org_id) NULLS NOT DISTINCT`,
	`DELETE FROM iam_assignments WHERE id IN (SELECT id FROM (SELECT id,row_number() OVER(PARTITION BY principal_type,principal_id,role_key,scope_type,scope_id ORDER BY (expires_at IS NULL) DESC,created_at DESC,id DESC) n FROM iam_assignments) d WHERE n>1)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS iam_assignments_nulls_unique ON iam_assignments(principal_type,principal_id,role_key,scope_type,scope_id) NULLS NOT DISTINCT`,
}
