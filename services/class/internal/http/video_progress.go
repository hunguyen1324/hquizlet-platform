package http

import (
	"bytes"
	"context"
	"database/sql"
	"log"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func databaseVideoProgress(db *sql.DB) func(context.Context, int64, int, string) {
	return func(ctx context.Context, id int64, percent int, stage string) {
		updateCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		var value any = percent
		if percent < 0 {
			value = nil
		}
		_, err := db.ExecContext(updateCtx, `INSERT INTO class_video_processing_progress(video_id,percent,stage) VALUES($1,$2,$3) ON CONFLICT(video_id) DO UPDATE SET percent=EXCLUDED.percent,stage=EXCLUDED.stage,updated_at=now()`, id, value, stage)
		if err != nil && ctx.Err() == nil {
			log.Printf("[class] video %d progress update: %v", id, err)
		}
	}
}

// Report only meaningful updates, at most once every two seconds within a
// stage. A new worker starts a fresh attempt rather than showing stale progress.
func (h *Handler) videoProgressReporter(ctx context.Context, id int64) func(int, string) {
	if h.videoProgress == nil {
		return nil
	}
	lastPercent, lastStage, lastWrite := -2, "", time.Time{}
	return func(percent int, stage string) {
		if percent > 99 {
			percent = 99
		}
		if percent < -1 {
			percent = -1
		}
		if stage == lastStage && (percent == lastPercent || time.Since(lastWrite) < 2*time.Second) {
			return
		}
		h.videoProgress(ctx, id, percent, stage)
		lastPercent, lastStage, lastWrite = percent, stage, time.Now()
	}
}

func probeHLSDuration(ctx context.Context, input string) float64 {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-protocol_whitelist", "file,crypto", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", input)
	var output boundedProbeOutput
	cmd.Stdout = &output
	if cmd.Run() != nil {
		return 0
	}
	seconds, err := strconv.ParseFloat(strings.TrimSpace(string(output.data)), 64)
	if err != nil || seconds <= 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return 0
	}
	return seconds
}

// FFmpeg -progress emits machine-readable records on stdout. Percent is based
// on processed media time, never elapsed wall time. Unknown duration stays null.
type ffmpegProgressWriter struct {
	pending  []byte
	duration float64
	report   func(int, string)
	highest  int
}

func (w *ffmpegProgressWriter) Write(data []byte) (int, error) {
	size := len(data)
	w.pending = append(w.pending, data...)
	for {
		i := bytes.IndexByte(w.pending, '\n')
		if i < 0 {
			break
		}
		line := strings.TrimSpace(string(w.pending[:i]))
		w.pending = w.pending[i+1:]
		name, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if name == "out_time_us" && w.duration > 0 {
			micros, err := strconv.ParseInt(value, 10, 64)
			if err != nil || micros < 0 {
				continue
			}
			fraction := math.Min(1, float64(micros)/1e6/w.duration)
			percent := 5 + int(fraction*85)
			if percent > w.highest {
				w.highest = percent
				if w.report != nil {
					w.report(percent, "encoding")
				}
			}
		}
	}
	if len(w.pending) > 64*1024 {
		w.pending = nil
	}
	return size, nil
}
