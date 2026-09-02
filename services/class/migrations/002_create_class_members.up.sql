-- 002: Create class_members table
CREATE TABLE IF NOT EXISTS class_members (
  id         BIGSERIAL    PRIMARY KEY,
  class_id   BIGINT       NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
  user_id    BIGINT       NOT NULL,
  role       VARCHAR(16)  NOT NULL DEFAULT 'student',
  joined_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  UNIQUE (class_id, user_id)
);

-- Constraints
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'class_members_role_check') THEN
    ALTER TABLE class_members ADD CONSTRAINT class_members_role_check CHECK (role IN ('owner', 'teacher', 'student'));
  END IF;
END $$;

-- Indexes
CREATE INDEX IF NOT EXISTS class_members_class_id_idx ON class_members(class_id);
CREATE INDEX IF NOT EXISTS class_members_user_id_idx ON class_members(user_id);
