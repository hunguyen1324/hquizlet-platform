package http

import (
	"context"
	"database/sql"
	"io"
	"log"
	"time"
)

// Durable database jobs survive restarts. A row lock prevents multiple replicas
// converting the same source and keeps deletion from racing the final publish.
func (h *Handler) RunVideoProcessing(ctx context.Context) {
	if len(h.hlsSecret) != 32 {
		return
	}
	for {
		found, err := h.processNextVideo(ctx)
		if err != nil {
			log.Printf("[class] video processing job: %v", err)
		}
		if found && err == nil {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

type closeOnEOF struct{ io.ReadCloser }

func (r closeOnEOF) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	if err != nil {
		r.ReadCloser.Close()
	}
	return n, err
}

func (h *Handler) processNextVideo(parent context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(parent, 12*time.Minute)
	defer cancel()
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var id int64
	var source, output string
	err = tx.QueryRowContext(ctx, `SELECT id,storage_key,hls_output_key FROM class_videos WHERE processing_state='processing' AND hls_output_key IS NOT NULL ORDER BY id FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&id, &source, &output)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	f, conversionErr := h.videoStore.Open(ctx, source)
	if conversionErr == nil {
		// Release the SFTP source connection after local spooling, before Put opens
		// another connection; works even when VIDEO_SFTP_MAX_CONNECTIONS=1.
		conversionErr = h.putHLS(ctx, output, closeOnEOF{f})
		f.Close()
	}
	if parent.Err() != nil {
		return true, parent.Err()
	}
	if conversionErr != nil {
		log.Printf("[class] video %d conversion failed: %v", id, conversionErr)
		// Mark failed and let the teacher delete/re-upload. A process crash instead
		// rolls back, leaving processing queued for the next startup.
		_, err = tx.ExecContext(ctx, `UPDATE class_videos SET processing_state='failed' WHERE id=$1`, id)
		if err != nil {
			return true, err
		}
		return true, tx.Commit()
	}
	_, err = tx.ExecContext(ctx, `UPDATE class_videos SET storage_key=$1,content_type=$2,processing_state='ready',hls_output_key=NULL WHERE id=$3`, output, hlsContentType, id)
	if err != nil {
		return true, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO class_video_deletions(storage_key) VALUES($1) ON CONFLICT DO NOTHING`, source)
	if err != nil {
		return true, err
	}
	return true, tx.Commit()
}
