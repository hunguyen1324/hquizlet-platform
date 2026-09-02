-- 003: Create class_study_sets table
-- NOTE: study_set_id is a cross-service reference to the Study service.
-- No FK constraint — existence is verified via internal HTTP call.
CREATE TABLE IF NOT EXISTS class_study_sets (
  class_id         BIGINT       NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
  study_set_id     BIGINT       NOT NULL,
  added_by_user_id BIGINT       NOT NULL,
  added_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  PRIMARY KEY (class_id, study_set_id)
);

-- Indexes
CREATE INDEX IF NOT EXISTS class_study_sets_class_id_idx ON class_study_sets(class_id);
CREATE INDEX IF NOT EXISTS class_study_sets_study_set_id_idx ON class_study_sets(study_set_id);
