package migration

func init() {
	migrations = append(migrations,
		`ALTER TABLE quiz_question ADD COLUMN IF NOT EXISTS srt_text TEXT NOT NULL DEFAULT ''`,
		`CREATE TABLE IF NOT EXISTS quiz_transcription_jobs(
 job_id TEXT PRIMARY KEY,study_set_id BIGINT NOT NULL REFERENCES study_sets(id) ON DELETE CASCADE,
 user_id BIGINT NOT NULL,audio_ref TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now())`,
		`CREATE INDEX IF NOT EXISTS quiz_transcription_jobs_lookup ON quiz_transcription_jobs(study_set_id,user_id,created_at DESC)`)
}
