package http

import (
	"archive/zip"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const hlsContentType = "application/vnd.apple.mpegurl"

var transcodeSlot = make(chan struct{}, 1)
var hlsSegmentName = regexp.MustCompile(`^segment[0-9]{6}\.ts$`)

func (h *Handler) ConfigureHLS(secret string) error {
	key, err := hex.DecodeString(secret)
	if err != nil || len(key) != 32 {
		return errors.New("VIDEO_HLS_MASTER_KEY must be 64 hex characters")
	}
	h.hlsSecret = key
	return nil
}

// Domain separation prevents a playback signature from being used as an AES key.
// The random object ID gives every video a separate key without placing keys on SFTP.
func hlsKey(secret []byte, object string) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte("hquizlet-hls-aes-v1\x00" + object))
	return mac.Sum(nil)[:16]
}

// Store a seekable, uncompressed ZIP of encrypted segments and the playlist.
// This keeps the existing single-object deletion/outbox lifecycle atomic.
func (h *Handler) putHLS(ctx context.Context, object string, input io.Reader) error {
	if len(h.hlsSecret) != 32 {
		return errors.New("HLS encryption is not configured")
	}
	select {
	case transcodeSlot <- struct{}{}:
		defer func() { <-transcodeSlot }()
	default:
		return errors.New("video converter busy")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	dir, err := os.MkdirTemp("", "hquizlet-hls-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	src, err := os.OpenFile(filepath.Join(dir, "input"), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = io.Copy(src, input)
	closeErr := src.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.WriteFile(filepath.Join(dir, "key"), hlsKey(h.hlsSecret, object), 0600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "keyinfo"), []byte("key\n"+filepath.Join(dir, "key")+"\n"), 0600); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "ffmpeg", "-nostdin", "-hide_banner", "-loglevel", "error", "-protocol_whitelist", "file,crypto", "-i", filepath.Join(dir, "input"), "-map", "0:v:0", "-map", "0:a:0?", "-c:v", "libx264", "-threads", "2", "-preset", "veryfast", "-crf", "23", "-vf", "scale=w='min(1280,iw)':h=-2", "-c:a", "aac", "-b:a", "128k", "-force_key_frames", "expr:gte(t,n_forced*6)", "-f", "hls", "-hls_time", "6", "-hls_playlist_type", "vod", "-hls_key_info_file", filepath.Join(dir, "keyinfo"), "-hls_segment_filename", filepath.Join(dir, "segment%06d.ts"), filepath.Join(dir, "index.m3u8"))
	// Bound disk use while FFmpeg is active, rather than only after it exits.
	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				entries, _ := os.ReadDir(dir)
				var size int64
				for _, e := range entries {
					info, err := e.Info()
					if err == nil {
						size += info.Size()
					}
				}
				if size > h.videoMaxBytes*4 {
					cancel()
					return
				}
			}
		}
	}()
	if err := cmd.Run(); err != nil {
		return errors.New("video conversion failed or timed out")
	}
	bundle, err := os.OpenFile(filepath.Join(dir, "bundle"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer bundle.Close()
	zw := zip.NewWriter(bundle)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var total int64
	for _, entry := range entries {
		name := entry.Name()
		if name != "index.m3u8" && !hlsSegmentName.MatchString(name) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		if total > h.videoMaxBytes*3 {
			return errors.New("converted video exceeds size limit")
		}
		dst, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
		if err != nil {
			return err
		}
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		_, err = io.Copy(dst, f)
		f.Close()
		if err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	if _, err := bundle.Seek(0, io.SeekStart); err != nil {
		return err
	}
	return h.videoStore.Put(ctx, object, bundle)
}

type seekReaderAt struct{ io.ReadSeeker }

func (s seekReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if _, err := s.Seek(off, io.SeekStart); err != nil {
		return 0, err
	}
	return io.ReadFull(s, p)
}

func rewriteHLS(raw, ticket string) (string, error) {
	lines := strings.Split(raw, "\n")
	encrypted := false
	resource := func(name string) string {
		return "class-video-hls?ticket=" + url.QueryEscape(ticket) + "&asset=" + url.QueryEscape(name)
	}
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#EXT-X-KEY:") {
			if !strings.HasPrefix(line, "#EXT-X-KEY:METHOD=AES-128,") || !strings.Contains(line, `URI="key"`) {
				return "", errors.New("invalid HLS encryption")
			}
			lines[i] = strings.Replace(line, `URI="key"`, `URI="`+resource("key")+`"`, 1)
			encrypted = true
		} else if line != "" && !strings.HasPrefix(line, "#") {
			if !hlsSegmentName.MatchString(line) {
				return "", errors.New("invalid HLS segment")
			}
			lines[i] = resource(line)
		}
	}
	if !encrypted || !strings.HasPrefix(raw, "#EXTM3U") {
		return "", errors.New("unencrypted HLS playlist")
	}
	return strings.Join(lines, "\n"), nil
}

func (h *Handler) serveHLS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if !h.privateVideosEnabled(w) {
		return
	}
	ticket, err := verifyVideoTicket(h.videoSecret, r.URL.Query().Get("ticket"), time.Now())
	if err != nil || ticket.UserID != userIDFromHeader(r) {
		WriteError(w, 401, "Vé phát không hợp lệ hoặc không thuộc phiên đăng nhập")
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
		WriteError(w, 403, "Bạn không còn là thành viên nhóm")
		return
	}
	object, ct, err := h.authorizedPrivateVideo(r.Context(), ticket.ClassID, ticket.VideoID, ticket.UserID, staff)
	if err == sql.ErrNoRows {
		WriteError(w, 404, "Video không tồn tại hoặc quyền đã bị thu hồi")
		return
	}
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	if ct != hlsContentType {
		WriteError(w, 404, "Video chưa được chuyển sang HLS")
		return
	}
	asset := r.URL.Query().Get("asset")
	if asset == "" {
		asset = "index.m3u8"
	}
	if asset == "key" {
		w.Header().Set("Content-Type", "application/octet-stream")
		if len(h.hlsSecret) != 32 {
			WriteError(w, 503, "HLS encryption is not configured")
			return
		}
		w.Write(hlsKey(h.hlsSecret, object))
		return
	}
	if asset != "index.m3u8" && !hlsSegmentName.MatchString(asset) {
		WriteError(w, 400, "Invalid asset")
		return
	}
	f, err := h.videoStore.Open(r.Context(), object)
	if err != nil {
		WriteError(w, 502, "Không đọc được kho video")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		WriteError(w, 502, "Không đọc được video")
		return
	}
	zr, err := zip.NewReader(seekReaderAt{f}, info.Size())
	if err != nil {
		WriteError(w, 502, "Video HLS không hợp lệ")
		return
	}
	for _, entry := range zr.File {
		if entry.Name == asset {
			if entry.Method != zip.Store || entry.UncompressedSize64 > uint64(h.videoMaxBytes*3) {
				WriteError(w, 502, "Invalid HLS asset")
				return
			}
			reader, err := entry.Open()
			if err != nil {
				WriteError(w, 502, "Không đọc được đoạn video")
				return
			}
			defer reader.Close()
			if asset == "index.m3u8" {
				raw, err := io.ReadAll(io.LimitReader(reader, 1024*1024))
				if err != nil {
					WriteError(w, 502, "Invalid playlist")
					return
				}
				playlist, err := rewriteHLS(string(raw), r.URL.Query().Get("ticket"))
				if err != nil {
					WriteError(w, 502, "Invalid playlist")
					return
				}
				w.Header().Set("Content-Type", hlsContentType)
				io.WriteString(w, playlist)
			} else {
				w.Header().Set("Content-Type", "video/mp2t")
				w.Header().Set("X-Accel-Buffering", "no")
				io.Copy(w, reader)
			}
			return
		}
	}
	WriteError(w, 404, "HLS asset not found")
}
