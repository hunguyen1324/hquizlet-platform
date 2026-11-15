package http

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type mediaItem struct {
	ID           int64     `json:"id"`
	OwnerID      int64     `json:"ownerId"`
	Kind         string    `json:"kind"`
	Title        string    `json:"title"`
	Body         string    `json:"body"`
	Source       string    `json:"source"`
	URL          string    `json:"url"`
	EmbedURL     string    `json:"embedUrl"`
	ThumbnailURL string    `json:"thumbnailUrl"`
	Visibility   string    `json:"visibility"`
	ViewerIDs    []int64   `json:"viewerIds"`
	CreatedAt    time.Time `json:"createdAt"`
	VideoIDs     []int64   `json:"videoIds"`
}

// Only provider-controlled embed URLs are rendered; never accept user iframe HTML.
func publicVideoSource(raw string) (source, embed, thumb string) {
	if id := youtubeID(raw); id != "" {
		return "youtube", "https://www.youtube.com/embed/" + id, "https://i.ytimg.com/vi/" + id + "/hqdefault.jpg"
	}
	u, e := url.Parse(strings.TrimSpace(raw))
	if e != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return
	}
	host := strings.ToLower(u.Hostname())
	if host == "www.tiktok.com" || host == "tiktok.com" {
		m := regexp.MustCompile(`^/@[^/]+/video/([0-9]{10,25})/?$`).FindStringSubmatch(u.Path)
		if len(m) == 2 {
			return "tiktok", "https://www.tiktok.com/player/v1/" + m[1], ""
		}
	}
	if host == "facebook.com" || host == "www.facebook.com" || host == "m.facebook.com" {
		if regexp.MustCompile(`^/(?:[^/]+/videos/[0-9]+|reel/[0-9]+)/?$`).MatchString(u.Path) || (strings.Trim(u.Path, "/") == "watch" && regexp.MustCompile(`^[0-9]+$`).MatchString(u.Query().Get("v"))) {
			return "facebook", "https://www.facebook.com/plugins/video.php?show_text=false&href=" + url.QueryEscape(u.String()), ""
		}
	}
	if strings.HasSuffix(strings.ToLower(u.Path), ".mp4") || strings.HasSuffix(strings.ToLower(u.Path), ".webm") {
		return "direct", u.String(), ""
	}
	return "", "", ""
}

const mediaColumns = `m.id,m.owner_user_id,m.kind,m.title,m.body,m.source,m.source_url,m.embed_url,m.thumbnail_url,m.visibility,m.viewer_ids,m.created_at`
const mediaAccess = `(m.visibility='public' OR m.owner_user_id=$1 OR (m.visibility='selected' AND m.viewer_ids @> $2::jsonb))`

func scanMedia(row interface{ Scan(...any) error }) (m mediaItem, err error) {
	var viewers []byte
	err = row.Scan(&m.ID, &m.OwnerID, &m.Kind, &m.Title, &m.Body, &m.Source, &m.URL, &m.EmbedURL, &m.ThumbnailURL, &m.Visibility, &viewers, &m.CreatedAt)
	if err == nil {
		err = json.Unmarshal(viewers, &m.ViewerIDs)
	}
	m.VideoIDs = []int64{}
	return
}
func (h *Handler) readMedia(r *http.Request, id, uid int64) (mediaItem, error) {
	viewer, _ := json.Marshal([]int64{uid})
	return scanMedia(h.db.QueryRowContext(r.Context(), `SELECT `+mediaColumns+` FROM media_items m WHERE `+mediaAccess+` AND m.id=$3`, uid, string(viewer), id))
}
func validMedia(in mediaItem) bool {
	if !validMangaName(in.Title, 200) || len([]rune(in.Body)) > 10000 || len(in.ViewerIDs) > 1000 {
		return false
	}
	if in.Visibility != "public" && in.Visibility != "private" && in.Visibility != "selected" {
		return false
	}
	for _, id := range in.ViewerIDs {
		if id < 1 {
			return false
		}
	}
	return in.Kind == "video" || in.Kind == "post" || in.Kind == "playlist"
}
func (h *Handler) mediaRouter(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	uid := userIDFromHeader(r)
	if uid < 1 {
		WriteError(w, 401, "authentication required")
		return
	}
	parts := PathParts(r.URL.Path, "/v1/media")
	if len(parts) == 1 && parts[0] == "upload" && r.Method == http.MethodPost {
		h.uploadPublicMedia(w, r, uid)
		return
	}
	if len(parts) == 0 {
		if r.Method == http.MethodGet {
			h.listMedia(w, r, uid)
			return
		}
		if r.Method != http.MethodPost {
			WriteError(w, 405, "method not allowed")
			return
		}
		var in mediaItem
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&in) != nil {
			WriteError(w, 422, "Dữ liệu không hợp lệ")
			return
		}
		if in.Visibility == "" {
			in.Visibility = "public"
		}
		if in.ViewerIDs == nil {
			in.ViewerIDs = []int64{}
		}
		if !validMedia(in) {
			WriteError(w, 422, "Tiêu đề, loại nội dung hoặc quyền xem không hợp lệ")
			return
		}
		if in.Kind == "video" {
			in.Source, in.EmbedURL, in.ThumbnailURL = publicVideoSource(in.URL)
			if in.Source == "" {
				WriteError(w, 422, "Dùng URL đầy đủ của YouTube, TikTok, Facebook hoặc file MP4/WebM HTTPS")
				return
			}
		}
		raw, _ := json.Marshal(in.ViewerIDs)
		e := h.db.QueryRowContext(r.Context(), `INSERT INTO media_items(owner_user_id,kind,title,body,source,source_url,embed_url,thumbnail_url,visibility,viewer_ids) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb) RETURNING id`, uid, in.Kind, strings.TrimSpace(in.Title), in.Body, in.Source, in.URL, in.EmbedURL, in.ThumbnailURL, in.Visibility, string(raw)).Scan(&in.ID)
		if e != nil {
			writeServiceError(w, r, e)
			return
		}
		WriteJSON(w, 201, map[string]any{"id": in.ID})
		return
	}
	id, e := strconv.ParseInt(parts[0], 10, 64)
	if e != nil || id < 1 {
		WriteError(w, 404, "Không có nội dung")
		return
	}
	item, e := h.readMedia(r, id, uid)
	if e != nil {
		if e == sql.ErrNoRows {
			WriteError(w, 404, "Không có nội dung hoặc bạn không được xem")
		} else {
			writeServiceError(w, r, e)
		}
		return
	}
	if len(parts) == 2 && parts[1] == "playback" && r.Method == http.MethodPost && item.Source == "upload" {
		if !h.privateVideosEnabled(w) {
			return
		}
		key := sha256.Sum256(append(append([]byte{}, h.videoSecret...), []byte("public-media")...))
		ticket := signVideoTicket(key[:], videoTicket{ClassID: 1, VideoID: id, UserID: uid, Expires: time.Now().Add(videoTicketTTL).Unix()})
		WriteJSON(w, 200, map[string]string{"url": "/v1/media-stream?ticket=" + url.QueryEscape(ticket)})
		return
	}
	if r.Method == http.MethodGet && len(parts) == 1 {
		if item.OwnerID != uid {
			item.ViewerIDs = []int64{}
		}
		WriteJSON(w, 200, item)
		return
	}
	if item.OwnerID != uid {
		WriteError(w, 403, "Chỉ chủ nội dung được thay đổi")
		return
	}
	if len(parts) == 3 && parts[1] == "items" && item.Kind == "playlist" && (r.Method == http.MethodPost || r.Method == http.MethodDelete) {
		h.changeMediaPlaylistItem(w, r, item, uid, parts[2])
		return
	}
	if len(parts) == 2 && parts[1] == "items" && r.Method == http.MethodPut && item.Kind == "playlist" {
		h.saveMediaPlaylist(w, r, item, uid)
		return
	}
	if len(parts) != 1 {
		WriteError(w, 404, "not found")
		return
	}
	if r.Method == http.MethodDelete {
		tx, e := h.db.BeginTx(r.Context(), nil)
		if e != nil {
			writeServiceError(w, r, e)
			return
		}
		defer tx.Rollback()
		_, e = tx.ExecContext(r.Context(), `INSERT INTO class_video_deletions(storage_key) SELECT storage_key FROM media_items WHERE id=$1 AND owner_user_id=$2 AND storage_key IS NOT NULL ON CONFLICT DO NOTHING`, id, uid)
		if e == nil {
			_, e = tx.ExecContext(r.Context(), `DELETE FROM media_items WHERE id=$1 AND owner_user_id=$2`, id, uid)
		}
		if e == nil {
			e = tx.Commit()
		}
		if e != nil {
			writeServiceError(w, r, e)
			return
		}
		w.WriteHeader(204)
		return
	}
	if r.Method == http.MethodPut {
		var in mediaItem
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&in) != nil {
			WriteError(w, 422, "Dữ liệu không hợp lệ")
			return
		}
		in.Kind = item.Kind
		if !validMedia(in) {
			WriteError(w, 422, "Thông tin không hợp lệ")
			return
		}
		if in.ViewerIDs == nil {
			in.ViewerIDs = []int64{}
		}
		raw, _ := json.Marshal(in.ViewerIDs)
		_, e = h.db.ExecContext(r.Context(), `UPDATE media_items SET title=$1,body=$2,visibility=$3,viewer_ids=$4::jsonb,updated_at=NOW() WHERE id=$5 AND owner_user_id=$6`, strings.TrimSpace(in.Title), in.Body, in.Visibility, string(raw), id, uid)
		if e != nil {
			writeServiceError(w, r, e)
			return
		}
		w.WriteHeader(204)
		return
	}
	WriteError(w, 405, "method not allowed")
}
func (h *Handler) listMedia(w http.ResponseWriter, r *http.Request, uid int64) {
	kind := r.URL.Query().Get("kind")
	if kind == "" {
		kind = "video"
	}
	limit, offset := queryLimitOffset(r, 60)
	playlist, _ := strconv.ParseInt(r.URL.Query().Get("playlistId"), 10, 64)
	if playlist > 0 {
		p, e := h.readMedia(r, playlist, uid)
		if e != nil || p.Kind != "playlist" {
			WriteError(w, 404, "Không có danh sách phát")
			return
		}
	}
	viewer, _ := json.Marshal([]int64{uid})
	rows, e := h.db.QueryContext(r.Context(), `SELECT `+mediaColumns+` FROM media_items m WHERE `+mediaAccess+` AND m.kind=$3 AND ($8=false OR m.owner_user_id=$1) AND (strpos(lower(m.title || ' ' || m.body),lower($4))>0) AND ($5=0 OR EXISTS(SELECT 1 FROM media_playlist_items p WHERE p.playlist_id=$5 AND p.video_id=m.id)) ORDER BY CASE WHEN $5>0 THEN (SELECT position FROM media_playlist_items p WHERE p.playlist_id=$5 AND p.video_id=m.id) ELSE 0 END,m.created_at DESC,m.id DESC LIMIT $6 OFFSET $7`, uid, string(viewer), kind, strings.TrimSpace(r.URL.Query().Get("search")), playlist, limit, offset, r.URL.Query().Get("mine") == "true")
	if e != nil {
		writeServiceError(w, r, e)
		return
	}
	defer rows.Close()
	items := []mediaItem{}
	for rows.Next() {
		m, e := scanMedia(rows)
		if e != nil {
			writeServiceError(w, r, e)
			return
		}
		if m.OwnerID != uid {
			m.ViewerIDs = []int64{}
		}
		items = append(items, m)
	}
	if e = rows.Err(); e != nil {
		writeServiceError(w, r, e)
		return
	}
	WriteJSON(w, 200, items)
}
func (h *Handler) saveMediaPlaylist(w http.ResponseWriter, r *http.Request, item mediaItem, uid int64) {
	var in struct {
		VideoIDs []int64 `json:"videoIds"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&in) != nil || len(in.VideoIDs) > 500 {
		WriteError(w, 422, "Danh sách tối đa 500 video")
		return
	}
	tx, e := h.db.BeginTx(r.Context(), nil)
	if e != nil {
		writeServiceError(w, r, e)
		return
	}
	defer tx.Rollback()
	var locked int64
	if e = tx.QueryRowContext(r.Context(), `SELECT id FROM media_items WHERE id=$1 AND owner_user_id=$2 FOR UPDATE`, item.ID, uid).Scan(&locked); e != nil {
		writeServiceError(w, r, e)
		return
	}
	if _, e = tx.ExecContext(r.Context(), `DELETE FROM media_playlist_items WHERE playlist_id=$1`, item.ID); e != nil {
		writeServiceError(w, r, e)
		return
	}
	viewer, _ := json.Marshal([]int64{uid})
	seen := map[int64]bool{}
	for position, id := range in.VideoIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		result, err := tx.ExecContext(r.Context(), `INSERT INTO media_playlist_items(playlist_id,video_id,position) SELECT $3,m.id,$4 FROM media_items m WHERE `+mediaAccess+` AND m.id=$5 AND m.kind='video'`, uid, string(viewer), item.ID, position, id)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		n, _ := result.RowsAffected()
		if n != 1 {
			WriteError(w, 422, "Video không tồn tại hoặc bạn không được xem")
			return
		}
	}
	if e = tx.Commit(); e != nil {
		writeServiceError(w, r, e)
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) uploadPublicMedia(w http.ResponseWriter, r *http.Request, uid int64) {
	if !h.privateVideosEnabled(w) {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.videoMaxBytes+(1<<20))
	reader, e := r.MultipartReader()
	if e != nil {
		WriteError(w, 400, "Cần dữ liệu multipart")
		return
	}
	metadata, e := reader.NextPart()
	if e != nil || metadata.FormName() != "metadata" {
		WriteError(w, 422, "Cần thông tin video trước file")
		return
	}
	var in mediaItem
	if json.NewDecoder(io.LimitReader(metadata, 64<<10)).Decode(&in) != nil {
		metadata.Close()
		WriteError(w, 422, "Thông tin video không hợp lệ")
		return
	}
	metadata.Close()
	in.Kind = "video"
	in.Title = strings.TrimSpace(in.Title)
	if in.Visibility == "" {
		in.Visibility = "public"
	}
	if !validMedia(in) {
		WriteError(w, 422, "Tiêu đề, quyền xem hoặc mô tả không hợp lệ")
		return
	}
	if in.ViewerIDs == nil || in.Visibility != "selected" {
		in.ViewerIDs = []int64{}
	}
	viewers, _ := json.Marshal(in.ViewerIDs)
	title := in.Title
	part, e := reader.NextPart()
	if e != nil || part.FormName() != "file" || part.FileName() == "" {
		WriteError(w, 400, "Cần file video")
		return
	}
	defer part.Close()
	src := bufio.NewReader(part)
	header, _ := src.Peek(512)
	ct := http.DetectContentType(header)
	ext := ".mp4"
	if ct == "video/webm" {
		ext = ".webm"
	} else if ct != "video/mp4" {
		WriteError(w, 415, "Chỉ hỗ trợ MP4 hoặc WebM")
		return
	}
	var random [16]byte
	if _, e = rand.Read(random[:]); e != nil {
		WriteError(w, 500, "Không tạo được mã file")
		return
	}
	key := hex.EncodeToString(random[:]) + ext
	committed := false
	defer func() {
		if !committed {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			h.videoStore.Delete(ctx, key)
		}
	}()
	limited := &boundedVideoReader{r: src, remaining: h.videoMaxBytes}
	if e = h.videoStore.Put(r.Context(), key, limited); e != nil {
		WriteError(w, 502, "Không tải được video hoặc vượt dung lượng cho phép")
		return
	}
	if _, e = reader.NextPart(); e != io.EOF {
		WriteError(w, 400, "Mỗi lần tải một video")
		return
	}
	var id int64
	e = h.db.QueryRowContext(r.Context(), `INSERT INTO media_items(owner_user_id,kind,title,source,storage_key,content_type,body,visibility,viewer_ids) VALUES($1,'video',$2,'upload',$3,$4,$5,$6,$7::jsonb) RETURNING id`, uid, title, key, ct, in.Body, in.Visibility, string(viewers)).Scan(&id)
	if e != nil {
		writeServiceError(w, r, e)
		return
	}
	committed = true
	WriteJSON(w, 201, map[string]any{"id": id})
}
func (h *Handler) streamPublicMedia(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	if !h.privateVideosEnabled(w) {
		return
	}
	keyHash := sha256.Sum256(append(append([]byte{}, h.videoSecret...), []byte("public-media")...))
	t, e := verifyVideoTicket(keyHash[:], r.URL.Query().Get("ticket"), time.Now())
	if e != nil || t.ClassID != 1 {
		WriteError(w, 401, "Phiên phát không hợp lệ hoặc hết hạn")
		return
	}
	viewer, _ := json.Marshal([]int64{t.UserID})
	var key, ct string
	e = h.db.QueryRowContext(r.Context(), `SELECT storage_key,content_type FROM media_items m WHERE `+mediaAccess+` AND m.id=$3 AND m.source='upload'`, t.UserID, string(viewer), t.VideoID).Scan(&key, &ct)
	if e != nil {
		WriteError(w, 404, "Video không tồn tại hoặc đã đổi quyền xem")
		return
	}
	f, e := h.videoStore.Open(r.Context(), key)
	if e != nil {
		WriteError(w, 502, "Không mở được video")
		return
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		WriteError(w, 502, "Không đọc được video")
		return
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, "video", info.ModTime(), f)
}

// Lock the playlist for single-item changes so two tabs cannot overwrite each other's additions.
func (h *Handler) changeMediaPlaylistItem(w http.ResponseWriter, r *http.Request, item mediaItem, uid int64, rawID string) {
	videoID, e := strconv.ParseInt(rawID, 10, 64)
	if e != nil || videoID < 1 {
		WriteError(w, 422, "Mã video không hợp lệ")
		return
	}
	tx, e := h.db.BeginTx(r.Context(), nil)
	if e != nil {
		writeServiceError(w, r, e)
		return
	}
	defer tx.Rollback()
	var locked int64
	if e = tx.QueryRowContext(r.Context(), `SELECT id FROM media_items WHERE id=$1 AND owner_user_id=$2 FOR UPDATE`, item.ID, uid).Scan(&locked); e != nil {
		writeServiceError(w, r, e)
		return
	}
	if r.Method == http.MethodDelete {
		_, e = tx.ExecContext(r.Context(), `DELETE FROM media_playlist_items WHERE playlist_id=$1 AND video_id=$2`, item.ID, videoID)
	} else {
		var exists bool
		if e = tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM media_playlist_items WHERE playlist_id=$1 AND video_id=$2)`, item.ID, videoID).Scan(&exists); e != nil {
			writeServiceError(w, r, e)
			return
		}
		if !exists {
			viewer, _ := json.Marshal([]int64{uid})
			result, err := tx.ExecContext(r.Context(), `INSERT INTO media_playlist_items(playlist_id,video_id,position) SELECT $3,m.id,COALESCE((SELECT max(position)+1 FROM media_playlist_items WHERE playlist_id=$3),0) FROM media_items m WHERE `+mediaAccess+` AND m.id=$4 AND m.kind='video' AND (SELECT count(*) FROM media_playlist_items WHERE playlist_id=$3)<500`, uid, string(viewer), item.ID, videoID)
			if err != nil {
				writeServiceError(w, r, err)
				return
			}
			n, _ := result.RowsAffected()
			if n != 1 {
				WriteError(w, 422, "Video không được xem hoặc danh sách đã đủ 500 video")
				return
			}
		}
	}
	if e == nil {
		e = tx.Commit()
	}
	if e != nil {
		writeServiceError(w, r, e)
		return
	}
	w.WriteHeader(204)
}
