-- Separate rows allow progress to commit while the worker owns the video job.
CREATE TABLE IF NOT EXISTS class_video_processing_progress (
    video_id BIGINT PRIMARY KEY REFERENCES class_videos(id) ON DELETE CASCADE,
    percent SMALLINT CHECK (percent BETWEEN 0 AND 99),
    stage TEXT NOT NULL CHECK (stage IN ('preparing','encoding','storing')),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
