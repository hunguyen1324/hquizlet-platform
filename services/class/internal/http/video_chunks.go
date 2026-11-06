package http

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/videostorage"
)

const videoChunkBytes = 8 * 1024 * 1024
const maxChunkedVideoBytes = 1024 * 1024 * 1024

func (h *Handler) chunkedVideo(w http.ResponseWriter, r *http.Request, classID, ownerID int64, parts []string) {
	if !h.privateVideosEnabled(w) {
		return
	}
	store, ok := h.videoStore.(videostorage.ChunkStore)
	if !ok || len(h.hlsSecret) != 32 {
		WriteError(w, 503, "Kho video chưa hỗ trợ tải chia phần")
		return
	}
	uid := userIDFromHeader(r)
	if len(parts) == 1 && r.Method == http.MethodPost {
		var in struct {
			Title, Audience, ContentType string
			ViewerIDs                    []int64
			Size                         int64
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32768)).Decode(&in); err != nil {
			WriteError(w, 400, "Thông tin tải lên không hợp lệ")
			return
		}
		in.Title = strings.TrimSpace(in.Title)
		if in.Size <= 0 || in.Size > h.videoMaxBytes || in.Size > maxChunkedVideoBytes {
			WriteError(w, 413, "Video vượt giới hạn tải lên (tối đa 1 GiB)")
			return
		}
		if in.Title == "" || len([]rune(in.Title)) > 200 || (in.ContentType != "video/mp4" && in.ContentType != "video/webm") {
			WriteError(w, 422, "Tiêu đề hoặc định dạng video không hợp lệ")
			return
		}
		ids, err := h.videoViewers(r.Context(), classID, ownerID, in.Audience, in.ViewerIDs)
		if err != nil {
			WriteError(w, 422, "Quyền xem không hợp lệ")
			return
		}
		// Serialize admission so concurrent sessions cannot reserve the same free disk.
		tx, err := h.db.BeginTx(r.Context(), nil)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		defer tx.Rollback()
		if _, err = tx.ExecContext(r.Context(), "SELECT pg_advisory_xact_lock(734811905)"); err != nil {
			writeServiceError(w, r, err)
			return
		}
		var reserved int64
		err = tx.QueryRowContext(r.Context(), `SELECT COALESCE(sum(size_bytes),0) FROM (SELECT size_bytes FROM class_video_uploads WHERE video_id IS NULL UNION ALL SELECT size_bytes FROM class_videos WHERE processing_state='processing') reservations`).Scan(&reserved)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		if free, diskErr := videoDiskFree(os.TempDir()); diskErr != nil || free < uint64((in.Size+reserved)*5+512*1024*1024) {
			WriteError(w, 507, "App không đủ dung lượng tạm để xử lý video này")
			return
		}
		if err = store.CheckSpace(r.Context(), in.Size+reserved); err != nil {
			log.Printf("[class] upload capacity: %v", err)
			WriteError(w, 507, "Kho video không đủ dung lượng hoặc chưa kết nối được")
			return
		}
		var random [16]byte
		if _, err = rand.Read(random[:]); err != nil {
			WriteError(w, 500, "Không tạo được phiên tải")
			return
		}
		id := hex.EncodeToString(random[:])
		ext := ".mp4"
		if in.ContentType == "video/webm" {
			ext = ".webm"
		}
		raw, _ := json.Marshal(ids)
		_, err = tx.ExecContext(r.Context(), `INSERT INTO class_video_uploads(id,class_id,user_id,title,audience,viewer_ids,storage_key,content_type,size_bytes) VALUES($1,$2,$3,$4,$5,$6::jsonb,$7,$8,$9)`, id, classID, uid, in.Title, in.Audience, string(raw), id+ext, in.ContentType, in.Size)
		if err == nil {
			err = tx.Commit()
		}
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		log.Printf("[class] upload started id=%s size=%d", id, in.Size)
		WriteJSON(w, 201, map[string]any{"uploadId": id, "chunkSize": videoChunkBytes})
		return
	}
	if len(parts) < 2 || len(parts) > 3 || len(parts[1]) != 32 {
		WriteError(w, 404, "Không tìm thấy phiên tải")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	defer tx.Rollback()
	var key, title, audience, ct string
	var viewers []byte
	var size, received int64
	var vid sql.NullInt64
	err = tx.QueryRowContext(r.Context(), `SELECT storage_key,title,audience,viewer_ids,content_type,size_bytes,received,video_id FROM class_video_uploads WHERE id=$1 AND class_id=$2 AND user_id=$3 AND expires_at>now() FOR UPDATE`, parts[1], classID, uid).Scan(&key, &title, &audience, &viewers, &ct, &size, &received, &vid)
	if err == sql.ErrNoRows {
		WriteError(w, 404, "Phiên tải không tồn tại hoặc đã hết hạn")
		return
	}
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	if len(parts) == 2 && r.Method == http.MethodGet {
		WriteJSON(w, 200, map[string]int64{"received": received})
		return
	}
	if len(parts) == 2 && r.Method == http.MethodDelete {
		if vid.Valid {
			WriteError(w, 409, "Video đã hoàn tất")
			return
		}
		if err = store.AbortUpload(r.Context(), key); err == nil {
			err = h.videoStore.Delete(r.Context(), key)
		}
		if err == nil {
			_, err = tx.ExecContext(r.Context(), `DELETE FROM class_video_uploads WHERE id=$1`, parts[1])
		}
		if err == nil {
			err = tx.Commit()
		}
		if err != nil {
			WriteError(w, 502, "Không dọn được phiên tải")
			return
		}
		WriteJSON(w, 200, map[string]bool{"ok": true})
		return
	}
	if len(parts) == 2 && r.Method == http.MethodPut && !vid.Valid {
		offset, parseErr := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 64)
		expected := int64(videoChunkBytes)
		if size-received < expected {
			expected = size - received
		}
		if parseErr != nil || offset != received || expected <= 0 {
			WriteError(w, 409, "Thứ tự phần tải không đúng")
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, videoChunkBytes+1))
		if err != nil || int64(len(body)) != expected {
			WriteError(w, 413, "Dung lượng phần tải không đúng")
			return
		}
		if offset == 0 && http.DetectContentType(body) != ct {
			WriteError(w, 415, "Nội dung file không đúng định dạng MP4/WebM")
			return
		}
		if err = store.WriteChunk(r.Context(), key, offset, bytes.NewReader(body)); err != nil {
			log.Printf("[class] upload chunk id=%s: %v", parts[1], err)
			WriteError(w, 502, "Không ghi được phần video, hãy thử lại")
			return
		}
		received += expected
		_, err = tx.ExecContext(r.Context(), `UPDATE class_video_uploads SET received=$2 WHERE id=$1`, parts[1], received)
		if err == nil {
			err = tx.Commit()
		}
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		log.Printf("[class] upload progress id=%s received=%d total=%d", parts[1], received, size)
		WriteJSON(w, 200, map[string]int64{"received": received})
		return
	}
	if len(parts) == 3 && parts[2] == "complete" && r.Method == http.MethodPost {
		var ids []int64
		_ = json.Unmarshal(viewers, &ids)
		if !vid.Valid {
			if received != size {
				WriteError(w, 409, "Video chưa tải đủ")
				return
			}
			if _, err = h.videoViewers(r.Context(), classID, ownerID, audience, ids); err != nil {
				WriteError(w, 422, "Quyền xem đã thay đổi")
				return
			}
			if err = store.FinishUpload(r.Context(), key, size); err != nil {
				WriteError(w, 502, "Không hoàn tất được file video")
				return
			}
			output := strings.TrimSuffix(key, ".mp4") + ".webm"
			if ct == "video/webm" {
				output = strings.TrimSuffix(key, ".webm") + ".mp4"
			}
			err = tx.QueryRowContext(r.Context(), `INSERT INTO class_videos(class_id,title,audience,viewer_ids,storage_key,content_type,size_bytes,processing_state,hls_output_key) VALUES($1,$2,$3,$4::jsonb,$5,$6,$7,'processing',$8) RETURNING id`, classID, title, audience, string(viewers), key, ct, size, output).Scan(&vid.Int64)
			if err == nil {
				_, err = tx.ExecContext(r.Context(), `UPDATE class_video_uploads SET video_id=$2 WHERE id=$1`, parts[1], vid.Int64)
			}
		}
		if err == nil {
			err = tx.Commit()
		}
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		WriteJSON(w, 202, classVideo{ID: vid.Int64, Title: title, Audience: audience, ViewerIDs: ids, Source: "sftp", SizeBytes: size, ProcessingState: "processing"})
		return
	}
	WriteError(w, 405, "Phương thức không hợp lệ")
}

// Expired incomplete sessions retain their reservation until their files are removed.
func (h *Handler) cleanupVideoUploads(ctx context.Context) error {
	store, ok := h.videoStore.(videostorage.ChunkStore)
	if !ok {
		return nil
	}
	rows, err := h.db.QueryContext(ctx, `SELECT id FROM class_video_uploads WHERE expires_at<now() ORDER BY expires_at LIMIT 10`)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		tx, err := h.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		var key string
		var vid sql.NullInt64
		err = tx.QueryRowContext(ctx, `SELECT storage_key,video_id FROM class_video_uploads WHERE id=$1 AND expires_at<now() FOR UPDATE SKIP LOCKED`, id).Scan(&key, &vid)
		if errors.Is(err, sql.ErrNoRows) {
			tx.Rollback()
			continue
		}
		if err == nil && !vid.Valid {
			err = store.AbortUpload(ctx, key)
			if err == nil {
				err = h.videoStore.Delete(ctx, key)
			}
		}
		if err == nil {
			_, err = tx.ExecContext(ctx, `DELETE FROM class_video_uploads WHERE id=$1`, id)
		}
		if err != nil {
			tx.Rollback()
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
