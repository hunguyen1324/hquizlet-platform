-- 001: Create classes table
CREATE TABLE IF NOT EXISTS classes (
  id           BIGSERIAL    PRIMARY KEY,
  owner_user_id BIGINT      NOT NULL,
  name         TEXT         NOT NULL,
  description  TEXT         NOT NULL DEFAULT '',
  invite_code  VARCHAR(8)   NOT NULL UNIQUE,
  max_members  INTEGER      NOT NULL DEFAULT 100,
  created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Constraints
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'classes_name_not_blank') THEN
    ALTER TABLE classes ADD CONSTRAINT classes_name_not_blank CHECK (trim(name) <> '');
  END IF;
END $$;

DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'classes_max_members_range') THEN
    ALTER TABLE classes ADD CONSTRAINT classes_max_members_range CHECK (max_members BETWEEN 1 AND 1000);
  END IF;
END $$;

-- Indexes
CREATE INDEX IF NOT EXISTS classes_owner_user_id_idx ON classes(owner_user_id);
