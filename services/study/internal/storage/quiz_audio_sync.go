package storage

import (
	"context"
	"log"
	"strings"
	"time"
)

// SyncPending also covers existing questions and both sync/async imports.
// A failed source is retained for retry; no question is silently stripped of audio.
func (a *QuizAudio) SyncPending(ctx context.Context) error {
	if a.backup == nil {
		return nil
	}
	rows, err := a.db.QueryContext(ctx, `
 WITH sources AS (
 SELECT study_set_id, audio_url AS source FROM quiz_question
 UNION
 SELECT study_set_id, value #>> '{}' FROM quiz_question,
 LATERAL jsonb_path_query(sub_questions::jsonb, '$.**.audioUrl') AS value
 ), pending AS (
 SELECT study_set_id, source FROM sources s
 WHERE source IS NOT NULL AND source <> '' AND NOT EXISTS (
 SELECT 1 FROM protected_quiz_audio a WHERE a.study_set_id=s.study_set_id
 AND (a.source_url=s.source OR 'private-audio:' || a.id=s.source) AND a.object_key IS NOT NULL
 )) SELECT study_set_id, source FROM pending ORDER BY study_set_id, source`)
	if err != nil {
		return err
	}
	type candidate struct {
		setID  int64
		source string
	}
	var pending []candidate
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.setID, &c.source); err != nil {
			rows.Close()
			return err
		}
		if a.allowed(c.source) || strings.HasPrefix(c.source, "private-audio:") {
			pending = append(pending, c)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, c := range pending {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if _, _, err := a.Open(ctx, c.setID, c.source); err != nil {
			// URLs can contain upstream credentials: do not log them or transport errors.
			log.Printf("[study] quiz audio sync failed for set %d; will retry", c.setID)
		}
	}
	return nil
}

func (a *QuizAudio) RunSync(ctx context.Context) {
	for {
		batch, cancel := context.WithTimeout(ctx, 5*time.Minute)
		err := a.SyncPending(batch)
		cancel()
		if err != nil {
			log.Printf("[study] quiz audio sync batch failed; will retry")
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Minute):
		}
	}
}
