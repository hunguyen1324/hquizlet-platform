package http

import (
	"bufio"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	classservice "github.com/hunguyen1324/hquizlet-platform/services/class/internal/service"
)

func (h *Handler) privateVideosEnabled(w http.ResponseWriter) bool {
	if h.videoStore == nil || len(h.videoSecret) < 32 {
		WriteError(w, 503, "Kho video riêng tư chưa được cấu hình")
		return false
	}
	return true
}

func (h *Handler) videoViewers(ctx context.Context, classID, ownerID int64, audience string, viewers []int64) ([]int64, error) {
	if (audience != "all" && audience != "selected") || len(viewers) > 1000 {
		return nil, errors.New("Quyền xem không hợp lệ")
	}
	ids := []int64{}
	if audience == "all" {
		return ids, nil
	}
	seen := map[int64]bool{}
	for _, id := range viewers {
		role, err := h.members.GetRole(ctx, classID, id)
		if err != nil {
			return nil, err
		}
		if id <= 0 || (role == "" && id != ownerID) {
			return nil, errors.New("Người xem phải là thành viên nhóm")
		}
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	return ids, nil
}

// Metadata is passed in the query to allow the multipart file to stream directly
// to SFTP. No multipart temp files or entire video buffers are created locally.
func (h *Handler) uploadPrivateVideo(w http.ResponseWriter, r *http.Request, classID, ownerID int64) {
	if !h.privateVideosEnabled(w) {
		return
	}
	if len(h.hlsSecret) != 32 {
		WriteError(w, 503, "Chưa cấu hình khóa mã hóa HLS")
		return
	}
	title, audience := strings.TrimSpace(r.URL.Query().Get("title")), r.URL.Query().Get("audience")
	var viewers []int64
	if raw := r.URL.Query().Get("viewerIds"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &viewers); err != nil {
			WriteError(w, 422, "Danh sách người xem không hợp lệ")
			return
		}
	}
	ids, err := h.videoViewers(r.Context(), classID, ownerID, audience, viewers)
	if err != nil {
		WriteError(w, 422, err.Error())
		return
	}
	if title == "" || len([]rune(title)) > 200 {
		WriteError(w, 422, "Tiêu đề không hợp lệ")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, h.videoMaxBytes+1024*1024)
	reader, err := r.MultipartReader()
	if err != nil {
		WriteError(w, 400, "Yêu cầu multipart không hợp lệ")
		return
	}
	part, err := reader.NextPart()
	if err != nil || part.FormName() != "file" || part.FileName() == "" {
		WriteError(w, 400, "Cần tải lên một file video")
		return
	}
	defer part.Close()
	src := bufio.NewReader(part)
	header, _ := src.Peek(512)
	contentType := http.DetectContentType(header)
	ext := ".mp4"
	if contentType == "video/webm" {
		ext = ".webm"
	} else if contentType != "video/mp4" {
		WriteError(w, 415, "Chỉ hỗ trợ video MP4 hoặc WebM")
		return
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		WriteError(w, 500, "Không tạo được mã video")
		return
	}
	key := hex.EncodeToString(random[:]) + ext
	limited := &boundedVideoReader{r: src, remaining: h.videoMaxBytes}
	if err := h.putHLS(r.Context(), key, limited); err != nil {
		log.Printf("[class] video upload failed: %v", err)
		var maxErr *http.MaxBytesError
		if errors.Is(err, errVideoTooLarge) || errors.As(err, &maxErr) {
			WriteError(w, 413, "Video vượt giới hạn tải lên")
		} else {
			WriteError(w, 502, "Không tải được video lên kho lưu trữ")
		}
		return
	}
	contentType = hlsContentType
	// Any failure after storing must remove the unreferenced object.
	committed := false
	defer func() {
		if !committed {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err := h.videoStore.Delete(ctx, key); err != nil {
				log.Printf("[class] cleanup orphan video %s failed: %v", key, err)
			}
		}
	}()
	if _, err := reader.NextPart(); err != io.EOF {
		WriteError(w, 400, "Chỉ tải một file video mỗi lần")
		return
	}
	// Recheck the uploader's role after a potentially long upload.
	cls, err := h.classes.GetByID(r.Context(), classID, userIDFromHeader(r))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	role, err := h.members.GetRole(r.Context(), classID, userIDFromHeader(r))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	if cls.OwnerUserID != userIDFromHeader(r) && role != "owner" && role != "teacher" {
		writeServiceError(w, r, classservice.ErrForbidden)
		return
	}
	viewJSON, _ := json.Marshal(ids)
	v := classVideo{Title: title, Audience: audience, ViewerIDs: ids, Source: "sftp", SizeBytes: limited.read}
	err = h.db.QueryRowContext(r.Context(), `INSERT INTO class_videos(class_id,title,audience,viewer_ids,storage_key,content_type,size_bytes) VALUES($1,$2,$3,$4::jsonb,$5,$6,$7) RETURNING id`, classID, title, audience, string(viewJSON), key, contentType, limited.read).Scan(&v.ID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	committed = true
	WriteJSON(w, 201, v)
}

var errVideoTooLarge = errors.New("video exceeds upload limit")

type boundedVideoReader struct {
	r               io.Reader
	remaining, read int64
}

func (b *boundedVideoReader) Read(p []byte) (int, error) {
	if b.remaining == 0 {
		var probe [1]byte
		n, err := b.r.Read(probe[:])
		if n > 0 {
			return 0, errVideoTooLarge
		}
		return 0, err
	}
	if int64(len(p)) > b.remaining {
		p = p[:b.remaining]
	}
	n, err := b.r.Read(p)
	b.remaining -= int64(n)
	b.read += int64(n)
	return n, err
}

func (h *Handler) updatePrivateVideo(w http.ResponseWriter, r *http.Request, classID, id, ownerID int64, title, audience string, viewers []int64) {
	if title == "" || len([]rune(title)) > 200 {
		WriteError(w, 422, "Tiêu đề không hợp lệ")
		return
	}
	ids, err := h.videoViewers(r.Context(), classID, ownerID, audience, viewers)
	if err != nil {
		WriteError(w, 422, err.Error())
		return
	}
	raw, _ := json.Marshal(ids)
	v := classVideo{ID: id, Title: title, Audience: audience, ViewerIDs: ids, Source: "sftp"}
	err = h.db.QueryRowContext(r.Context(), `UPDATE class_videos SET title=$1,audience=$2,viewer_ids=$3::jsonb WHERE class_id=$4 AND id=$5 AND storage_key IS NOT NULL RETURNING size_bytes`, title, audience, string(raw), classID, id).Scan(&v.SizeBytes)
	if err != nil {
		if err == sql.ErrNoRows {
			WriteError(w, 404, "video not found")
		} else {
			writeServiceError(w, r, err)
		}
		return
	}
	WriteJSON(w, 200, v)
}

type videoTicket struct {
	UserID  int64 `json:"u"`
	ClassID int64 `json:"c"`
	VideoID int64 `json:"v"`
	Expires int64 `json:"e"`
}

const videoTicketTTL = 5 * time.Minute

func signVideoTicket(secret []byte, ticket videoTicket) string {
	body, _ := json.Marshal(ticket)
	payload := base64.RawURLEncoding.EncodeToString(body)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
func verifyVideoTicket(secret []byte, raw string, now time.Time) (videoTicket, error) {
	var ticket videoTicket
	parts := strings.Split(raw, ".")
	if len(secret) < 32 || len(raw) > 1024 || len(parts) != 2 {
		return ticket, errors.New("invalid video ticket")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ticket, err
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(parts[0]))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return ticket, errors.New("invalid video signature")
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ticket, err
	}
	if err = json.Unmarshal(body, &ticket); err != nil {
		return ticket, err
	}
	if ticket.UserID <= 0 || ticket.ClassID <= 0 || ticket.VideoID <= 0 || ticket.Expires <= now.Unix() || ticket.Expires > now.Add(videoTicketTTL).Unix() {
		return ticket, errors.New("expired or invalid video ticket")
	}
	return ticket, nil
}

func (h *Handler) authorizedPrivateVideo(ctx context.Context, classID, id, uid int64, staff bool) (key, contentType string, err error) {
	viewer, _ := json.Marshal([]int64{uid})
	err = h.db.QueryRowContext(ctx, `SELECT storage_key,content_type FROM class_videos WHERE class_id=$1 AND id=$2 AND storage_key IS NOT NULL AND ($3 OR audience='all' OR viewer_ids @> $4::jsonb)`, classID, id, staff, string(viewer)).Scan(&key, &contentType)
	return
}

func (h *Handler) privateVideoPlayback(w http.ResponseWriter, r *http.Request, classID, id, uid int64, staff bool) {
	if !h.privateVideosEnabled(w) {
		return
	}
	_, contentType, err := h.authorizedPrivateVideo(r.Context(), classID, id, uid, staff)
	if err != nil {
		if err == sql.ErrNoRows {
			WriteError(w, 404, "Video không tồn tại hoặc bạn không có quyền xem")
		} else {
			writeServiceError(w, r, err)
		}
		return
	}
	expires := time.Now().Add(videoTicketTTL)
	ticket := signVideoTicket(h.videoSecret, videoTicket{uid, classID, id, expires.Unix()})
	path := "/v1/class-video-stream"
	if contentType == hlsContentType {
		path = "/v1/class-video-hls"
	}
	WriteJSON(w, 200, map[string]any{"path": path + "?ticket=" + url.QueryEscape(ticket), "expiresAt": expires})
}

func (h *Handler) streamPrivateVideo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if !h.privateVideosEnabled(w) {
		return
	}
	ticket, err := verifyVideoTicket(h.videoSecret, r.URL.Query().Get("ticket"), time.Now())
	if err != nil {
		WriteError(w, 401, "Vé phát video hết hạn hoặc không hợp lệ")
		return
	}
	cls, err := h.classes.GetByID(r.Context(), ticket.ClassID, ticket.UserID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	role, err := h.members.GetRole(r.Context(), ticket.ClassID, ticket.UserID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	staff := cls.OwnerUserID == ticket.UserID || role == "owner" || role == "teacher"
	if !staff && role == "" {
		writeServiceError(w, r, classservice.ErrForbidden)
		return
	}
	key, contentType, err := h.authorizedPrivateVideo(r.Context(), ticket.ClassID, ticket.VideoID, ticket.UserID, staff)
	if err != nil {
		if err == sql.ErrNoRows {
			WriteError(w, 404, "Video không tồn tại hoặc bạn không còn quyền xem")
		} else {
			writeServiceError(w, r, err)
		}
		return
	}
	if contentType == hlsContentType {
		WriteError(w, 403, "Video HLS yêu cầu phiên đăng nhập")
		return
	}
	f, err := h.videoStore.Open(r.Context(), key)
	if err != nil {
		WriteError(w, 502, "Không đọc được video từ kho lưu trữ")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		WriteError(w, 502, "File video không hợp lệ")
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Content-Disposition", `inline; filename="video-`+strconv.FormatInt(ticket.VideoID, 10)+`"`)
	// ServeContent handles HEAD, byte ranges, suffix ranges and 416 without downloading the whole video.
	http.ServeContent(w, r, key, info.ModTime(), f)
}
