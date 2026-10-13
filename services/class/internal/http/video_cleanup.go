package http

import (
	"context"
	"log"
	"time"
)

// RunVideoCleanup drains the durable deletion queue. The DB trigger also queues
// files removed by ON DELETE CASCADE, so deleting a class doesn't leak disk space.
func (h *Handler) RunVideoCleanup(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		if err := h.cleanupVideos(ctx); err != nil {
			log.Printf("[class] video cleanup: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (h *Handler) cleanupVideos(ctx context.Context) error {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	rows, err := h.db.QueryContext(queryCtx, `SELECT storage_key FROM class_video_deletions ORDER BY created_at LIMIT 25`)
	if err != nil {
		cancel()
		return err
	}
	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			rows.Close()
			cancel()
			return err
		}
		keys = append(keys, key)
	}
	err = rows.Err()
	rows.Close()
	cancel()
	if err != nil {
		return err
	}
	for _, key := range keys {
		opCtx, opCancel := context.WithTimeout(ctx, 20*time.Second)
		err := h.videoStore.Delete(opCtx, key)
		if err == nil {
			_, err = h.db.ExecContext(opCtx, `DELETE FROM class_video_deletions WHERE storage_key=$1`, key)
		}
		opCancel()
		// Leave failed work queued for the next minute instead of dropping it.
		if err != nil {
			return err
		}
	}
	return nil
}
