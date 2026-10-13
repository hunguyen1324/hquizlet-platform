-- 009: per-class leaderboard visibility for students (dashboard plan CLS-05).
-- Default false: teachers opt in per class.
ALTER TABLE classes ADD COLUMN IF NOT EXISTS show_leaderboard BOOLEAN NOT NULL DEFAULT false;
