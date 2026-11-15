package http

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/manga"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/videostorage"
)

const maxMangaImage = 12 << 20

type mangaVolume struct {
	ID         int64  `json:"id"`
	OwnerID    int64  `json:"ownerId"`
	SourceUUID string `json:"sourceUuid"`
	Title      string `json:"title"`
	Volume     string `json:"volume"`
	Visibility string `json:"visibility"`
	Status     string `json:"status"`
	PageCount  int    `json:"pageCount"`
	Uploaded   int    `json:"uploaded"`
	CanEdit    bool   `json:"canEdit"`
	CanDelete  bool   `json:"canDelete"`
	ClassID    int64  `json:"classId"`
	SeriesID   int64  `json:"seriesId"`
}

func (h *Handler) ConfigureManga(store videostorage.Store) { h.mangaStore = store }
func validMangaName(s string, max int) bool {
	return strings.TrimSpace(s) != "" && len([]rune(s)) <= max && !strings.ContainsRune(s, 0)
}
func (h *Handler) mangaRouter(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	uid := userIDFromHeader(r)
	if uid < 1 {
		WriteError(w, 401, "authentication required")
		return
	}
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/manga"), "/")
	parts := []string{}
	if rest != "" {
		parts = strings.Split(rest, "/")
	}
	if len(parts) == 0 {
		h.mangaCollection(w, r, uid)
		return
	}
	if len(parts) == 1 && parts[0] == "inspect" && r.Method == http.MethodPost {
		var in struct {
			HTMLSource string `json:"htmlSource"`
			SourceName string `json:"sourceName"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 12<<20)).Decode(&in) != nil {
			WriteError(w, 422, "HTML không hợp lệ hoặc quá lớn")
			return
		}
		parsed, err := manga.ParseHTML(in.HTMLSource, in.SourceName)
		if err != nil {
			WriteError(w, 422, err.Error())
			return
		}
		WriteJSON(w, 200, parsed)
		return
	}
	id, e := strconv.ParseInt(parts[0], 10, 64)
	if e != nil || id < 1 {
		WriteError(w, 404, "Không có truyện")
		return
	}
	var v mangaVolume
	e = h.db.QueryRowContext(r.Context(), `SELECT id,owner_user_id,source_uuid,title,volume,visibility,status,page_count,COALESCE(class_id,0),COALESCE(series_id,0) FROM manga_volumes WHERE id=$1`, id).Scan(&v.ID, &v.OwnerID, &v.SourceUUID, &v.Title, &v.Volume, &v.Visibility, &v.Status, &v.PageCount, &v.ClassID, &v.SeriesID)
	if e == sql.ErrNoRows {
		WriteError(w, 404, "Không có truyện hoặc bạn không được xem")
		return
	}
	if e != nil {
		writeServiceError(w, r, e)
		return
	}
	v.CanEdit, v.CanDelete = v.OwnerID == uid, v.OwnerID == uid
	if v.ClassID > 0 {
		if h.access == nil {
			WriteError(w, 503, "Phân quyền lớp chưa sẵn sàng")
			return
		}
		if !h.requireClassAction(w, r, v.ClassID, "class-manga:Read") {
			return
		}
		v.CanEdit = h.hasClassAction(r, v.ClassID, "class-manga:Update")
		v.CanDelete = h.hasClassAction(r, v.ClassID, "class-manga:Delete")
		if v.OwnerID != uid && (v.Status != "ready" || v.Visibility == "private") {
			WriteError(w, 404, "Không có truyện hoặc bạn không được xem")
			return
		}
	} else if v.OwnerID != uid && (v.Visibility != "shared" || v.Status != "ready") {
		WriteError(w, 404, "Không có truyện hoặc bạn không được xem")
		return
	}
	if len(parts) == 2 && parts[1] == "move" && r.Method == http.MethodPost {
		var in struct {
			ClassID int64 `json:"classId"`
		}
		if v.OwnerID != uid || v.ClassID != 0 {
			WriteError(w, 403, "Chỉ chủ truyện được đưa truyện cũ vào lớp")
			return
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&in) != nil || in.ClassID < 1 {
			WriteError(w, 422, "Chọn lớp học hợp lệ")
			return
		}
		if h.access == nil {
			WriteError(w, 503, "Phân quyền chưa sẵn sàng")
			return
		}
		if !h.requireClassAction(w, r, in.ClassID, "class-manga:Create") {
			return
		}
		tx, err := h.db.BeginTx(r.Context(), nil)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		defer tx.Rollback()
		seriesID, _, err := resolveMangaSeries(r.Context(), tx, uid, in.ClassID, 0, v.Title)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		_, err = tx.ExecContext(r.Context(), `UPDATE manga_volumes SET class_id=$1,visibility='shared',series_id=$4,updated_at=NOW() WHERE id=$2 AND owner_user_id=$3 AND class_id IS NULL`, in.ClassID, id, uid, seriesID)
		if err == nil {
			err = tx.Commit()
		}
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		w.WriteHeader(204)
		return
	}
	if len(parts) == 2 && parts[1] == "export.html" && r.Method == http.MethodGet {
		h.exportMangaHTML(w, r, v)
		return
	}
	if r.Method != http.MethodGet && ((r.Method == http.MethodDelete && !v.CanDelete) || (r.Method != http.MethodDelete && !v.CanEdit && !(v.Status == "importing" && v.OwnerID == uid && v.ClassID > 0 && h.hasClassAction(r, v.ClassID, "class-manga:Create")))) {
		WriteError(w, 403, "Chỉ chủ truyện được sửa")
		return
	}
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			rows, e := h.db.QueryContext(r.Context(), `SELECT page_index FROM manga_pages WHERE volume_id=$1 ORDER BY page_index`, id)
			if e != nil {
				writeServiceError(w, r, e)
				return
			}
			defer rows.Close()
			indices := []int{}
			for rows.Next() {
				var n int
				if e = rows.Scan(&n); e != nil {
					writeServiceError(w, r, e)
					return
				}
				indices = append(indices, n)
			}
			if e = rows.Err(); e != nil {
				writeServiceError(w, r, e)
				return
			}
			v.Uploaded = len(indices)
			WriteJSON(w, 200, map[string]any{"volume": v, "pages": indices})
		case http.MethodPut:
			var in struct {
				Title      string `json:"title"`
				Volume     string `json:"volume"`
				Visibility string `json:"visibility"`
				SeriesID   int64  `json:"seriesId"`
			}
			if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&in) != nil || !validMangaName(in.Title, 200) || len([]rune(in.Volume)) > 200 || (in.Visibility != "private" && in.Visibility != "shared") {
				WriteError(w, 422, "Tên truyện hoặc chế độ chia sẻ không hợp lệ")
				return
			}
			tx, err := h.db.BeginTx(r.Context(), nil)
			if err != nil {
				writeServiceError(w, r, err)
				return
			}
			defer tx.Rollback()
			// The uploader owns an automatically created series; classroom editors may use a visible existing series.
			target := in.SeriesID
			if target == 0 && strings.TrimSpace(in.Title) == v.Title {
				target = v.SeriesID
			}
			seriesID, title, err := resolveMangaSeries(r.Context(), tx, uid, v.ClassID, target, strings.TrimSpace(in.Title))
			if err == sql.ErrNoRows {
				WriteError(w, 422, "Không được chuyển tập vào truyện này")
				return
			}
			if err != nil {
				writeServiceError(w, r, err)
				return
			}
			_, err = tx.ExecContext(r.Context(), `UPDATE manga_volumes SET title=$2,volume=$3,visibility=$4,series_id=$5,updated_at=NOW() WHERE id=$1`, id, title, strings.TrimSpace(in.Volume), in.Visibility, seriesID)
			if err == nil {
				err = tx.Commit()
			}
			if err != nil {
				writeServiceError(w, r, err)
				return
			}
			w.WriteHeader(204)
		case http.MethodDelete:
			tx, e := h.db.BeginTx(r.Context(), nil)
			if e != nil {
				writeServiceError(w, r, e)
				return
			}
			defer tx.Rollback()
			var locked int64
			if e = tx.QueryRowContext(r.Context(), `SELECT id FROM manga_volumes WHERE id=$1 FOR UPDATE`, id).Scan(&locked); e != nil {
				writeServiceError(w, r, e)
				return
			}
			if _, e = tx.ExecContext(r.Context(), `INSERT INTO manga_image_gc(image_key) SELECT image_key FROM manga_pages WHERE volume_id=$1 ON CONFLICT DO NOTHING`, id); e == nil {
				_, e = tx.ExecContext(r.Context(), `DELETE FROM manga_volumes WHERE id=$1`, id)
			}
			if e == nil {
				e = tx.Commit()
			}
			if e != nil {
				writeServiceError(w, r, e)
				return
			}
			w.WriteHeader(204)
		default:
			WriteError(w, 405, "method not allowed")
		}
		return
	}
	if len(parts) == 2 && parts[1] == "complete" && r.Method == http.MethodPost {
		result, e := h.db.ExecContext(r.Context(), `UPDATE manga_volumes SET status='ready',updated_at=NOW() WHERE id=$1 AND owner_user_id=$2 AND page_count=(SELECT count(*) FROM manga_pages WHERE volume_id=$1)`, id, uid)
		if e != nil {
			writeServiceError(w, r, e)
			return
		}
		n, _ := result.RowsAffected()
		if n == 0 {
			WriteError(w, 409, "Chưa đủ ảnh trang để hoàn tất nhập truyện")
			return
		}
		w.WriteHeader(204)
		return
	}
	if len(parts) < 3 || parts[1] != "pages" || len(parts) > 4 {
		WriteError(w, 404, "not found")
		return
	}
	n, e := strconv.Atoi(parts[2])
	if e != nil || n < 0 || n >= v.PageCount {
		WriteError(w, 404, "Không có trang này")
		return
	}
	if len(parts) == 3 && r.Method == http.MethodPost {
		h.uploadMangaPage(w, r, v, n)
		return
	}
	if len(parts) == 3 && r.Method == http.MethodPut {
		h.editMangaOCR(w, r, v, n)
		return
	}
	if r.Method != http.MethodGet {
		WriteError(w, 405, "method not allowed")
		return
	}
	if len(parts) == 4 {
		if parts[3] != "image" {
			WriteError(w, 404, "not found")
			return
		}
		if h.mangaStore == nil {
			WriteError(w, 503, "Kho ảnh chưa được cấu hình")
			return
		}
		var key, mime string
		e = h.db.QueryRowContext(r.Context(), `SELECT image_key,image_type FROM manga_pages WHERE volume_id=$1 AND page_index=$2`, id, n).Scan(&key, &mime)
		if e == sql.ErrNoRows {
			WriteError(w, 404, "Chưa có ảnh trang")
			return
		}
		if e != nil {
			writeServiceError(w, r, e)
			return
		}
		file, e := h.mangaStore.Open(r.Context(), key)
		if e != nil {
			WriteError(w, 503, "Chưa đọc được ảnh, vui lòng thử lại")
			return
		}
		defer file.Close()
		w.Header().Set("Content-Type", mime)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeContent(w, r, "page", time.Time{}, file)
		return
	}
	var p manga.Page
	var raw []byte
	var revision int
	e = h.db.QueryRowContext(r.Context(), `SELECT width,height,blocks,revision FROM manga_pages WHERE volume_id=$1 AND page_index=$2`, id, n).Scan(&p.Width, &p.Height, &raw, &revision)
	if e == sql.ErrNoRows {
		WriteError(w, 404, "Chưa có trang")
		return
	}
	if e != nil {
		writeServiceError(w, r, e)
		return
	}
	if json.Unmarshal(raw, &p.Blocks) != nil {
		WriteError(w, 500, "Dữ liệu OCR bị lỗi")
		return
	}
	WriteJSON(w, 200, map[string]any{"page": p, "revision": revision})
}
func (h *Handler) mangaCollection(w http.ResponseWriter, r *http.Request, uid int64) {
	classID, _ := strconv.ParseInt(r.URL.Query().Get("classId"), 10, 64)
	if classID > 0 {
		if h.access == nil {
			WriteError(w, 503, "Phân quyền lớp chưa sẵn sàng")
			return
		}
		action := "class-manga:Read"
		if r.Method == http.MethodPost {
			action = "class-manga:Create"
		}
		if !h.requireClassAction(w, r, classID, action) {
			return
		}
	}

	if r.Method == http.MethodGet {
		rows, e := h.db.QueryContext(r.Context(), `SELECT v.id,v.owner_user_id,v.source_uuid,v.title,v.volume,v.visibility,v.status,v.page_count,COALESCE(v.series_id,0),(SELECT count(*) FROM manga_pages p WHERE p.volume_id=v.id) FROM manga_volumes v WHERE COALESCE(v.class_id,0)=$2 AND (v.owner_user_id=$1 OR (v.visibility='shared' AND v.status='ready')) ORDER BY v.updated_at DESC,v.id DESC LIMIT 200`, uid, classID)
		if e != nil {
			writeServiceError(w, r, e)
			return
		}
		defer rows.Close()
		volumes := []mangaVolume{}
		for rows.Next() {
			var v mangaVolume
			if e = rows.Scan(&v.ID, &v.OwnerID, &v.SourceUUID, &v.Title, &v.Volume, &v.Visibility, &v.Status, &v.PageCount, &v.SeriesID, &v.Uploaded); e != nil {
				writeServiceError(w, r, e)
				return
			}
			v.ClassID = classID
			v.CanEdit, v.CanDelete = v.OwnerID == uid, v.OwnerID == uid
			if classID > 0 {
				v.CanEdit = h.hasClassAction(r, classID, "class-manga:Update")
				v.CanDelete = h.hasClassAction(r, classID, "class-manga:Delete")
			}
			volumes = append(volumes, v)
		}
		if e = rows.Err(); e != nil {
			writeServiceError(w, r, e)
			return
		}
		WriteJSON(w, 200, volumes)
		return
	}
	if r.Method != http.MethodPost {
		WriteError(w, 405, "method not allowed")
		return
	}
	if h.mangaStore == nil {
		WriteError(w, 503, "Kho ảnh chưa được cấu hình")
		return
	}
	var in struct {
		Title      string `json:"title"`
		Volume     string `json:"volume"`
		SourceUUID string `json:"sourceUuid"`
		PageCount  int    `json:"pageCount"`
		HTMLSource string `json:"htmlSource"`
		SourceName string `json:"sourceName"`
		SeriesID   int64  `json:"seriesId"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 12<<20)).Decode(&in) != nil {
		WriteError(w, 422, "Thông tin tập truyện không hợp lệ")
		return
	}
	if in.HTMLSource != "" {
		parsed, e := manga.ParseHTML(in.HTMLSource, in.SourceName)
		if e != nil {
			WriteError(w, 422, e.Error())
			return
		}
		in.SourceUUID = parsed.SourceUUID
		in.PageCount = len(parsed.Pages)
	}
	if in.SeriesID < 0 || !validMangaName(in.Title, 200) || len([]rune(in.Volume)) > 200 || !validMangaName(in.SourceUUID, 100) || in.PageCount < 1 || in.PageCount > 1000 || len([]rune(in.SourceName)) > 255 || strings.ContainsAny(in.SourceName, "/\\\r\n\x00") {
		WriteError(w, 422, "Thông tin tập truyện không hợp lệ")
		return
	}
	tx, e := h.db.BeginTx(r.Context(), nil)
	if e != nil {
		writeServiceError(w, r, e)
		return
	}
	defer tx.Rollback()
	seriesID, title, e := resolveMangaSeries(r.Context(), tx, uid, classID, in.SeriesID, strings.TrimSpace(in.Title))
	if e == sql.ErrNoRows {
		WriteError(w, 422, "Truyện không tồn tại hoặc không thuộc không gian này")
		return
	}
	if e != nil {
		writeServiceError(w, r, e)
		return
	}
	var id int64
	e = tx.QueryRowContext(r.Context(), `INSERT INTO manga_volumes(owner_user_id,source_uuid,title,volume,page_count,html_source,source_name,class_id,visibility,series_id) VALUES($1,$2,$3,$4,$5,$6,$7,NULLIF($8,0),'shared',$9) ON CONFLICT DO NOTHING RETURNING id`, uid, in.SourceUUID, title, strings.TrimSpace(in.Volume), in.PageCount, in.HTMLSource, in.SourceName, classID, seriesID).Scan(&id)
	if e == sql.ErrNoRows {
		var status string
		var count int
		var existingSeries int64
		e = tx.QueryRowContext(r.Context(), `SELECT id,status,page_count,COALESCE(series_id,0) FROM manga_volumes WHERE owner_user_id=$1 AND source_uuid=$2 AND COALESCE(class_id,0)=$3`, uid, in.SourceUUID, classID).Scan(&id, &status, &count, &existingSeries)
		if e == nil && (status != "importing" || count != in.PageCount || existingSeries != seriesID) {
			WriteError(w, 409, "Tập truyện đã tồn tại; mở truyện để sửa OCR thay vì nhập đè")
			return
		}
	}
	if e != nil {
		writeServiceError(w, r, e)
		return
	}
	if e = tx.Commit(); e != nil {
		writeServiceError(w, r, e)
		return
	}
	WriteJSON(w, 201, map[string]any{"id": id, "seriesId": seriesID})
}
func (h *Handler) uploadMangaPage(w http.ResponseWriter, r *http.Request, v mangaVolume, n int) {
	if h.mangaStore == nil {
		WriteError(w, 503, "Kho ảnh chưa được cấu hình")
		return
	}
	if v.Status != "importing" {
		WriteError(w, 409, "Truyện đã hoàn tất, chỉ được sửa chữ OCR")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxMangaImage+(2<<20))
	if e := r.ParseMultipartForm(maxMangaImage + (2 << 20)); e != nil {
		WriteError(w, 413, "Ảnh tối đa 12 MiB và dữ liệu OCR tối đa 1 MiB")
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	var p manga.Page
	raw := []byte(r.FormValue("page"))
	if len(raw) > 1<<20 || json.Unmarshal(raw, &p) != nil || manga.Validate(&p) != nil {
		WriteError(w, 422, "Dữ liệu trang OCR không hợp lệ")
		return
	}
	f, _, e := r.FormFile("image")
	if e != nil {
		WriteError(w, 422, "Thiếu ảnh trang")
		return
	}
	defer f.Close()
	data, e := io.ReadAll(io.LimitReader(f, maxMangaImage+1))
	if e != nil || len(data) > maxMangaImage {
		WriteError(w, 413, "Ảnh tối đa 12 MiB")
		return
	}
	cfg, format, e := image.DecodeConfig(bytes.NewReader(data))
	if e != nil || (format != "jpeg" && format != "png") || cfg.Width != p.Width || cfg.Height != p.Height {
		WriteError(w, 422, "Ảnh phải là JPEG/PNG và khớp kích thước trong Mokuro")
		return
	}
	ext, mime := "jpg", "image/jpeg"
	if format == "png" {
		ext, mime = "png", "image/png"
	}
	var keyBytes [16]byte
	if _, e = rand.Read(keyBytes[:]); e != nil {
		writeServiceError(w, r, e)
		return
	}
	key := hex.EncodeToString(keyBytes[:]) + "." + ext
	if e = h.mangaStore.Put(r.Context(), key, bytes.NewReader(data)); e != nil {
		WriteError(w, 503, "Không lưu được ảnh vào kho riêng")
		return
	}
	tx, e := h.db.BeginTx(r.Context(), nil)
	if e == nil {
		defer tx.Rollback()
		var status string
		e = tx.QueryRowContext(r.Context(), `SELECT status FROM manga_volumes WHERE id=$1 AND owner_user_id=$2 FOR UPDATE`, v.ID, v.OwnerID).Scan(&status)
		if e == nil && status != "importing" {
			e = fmt.Errorf("import closed")
		}
		if e == nil {
			blocks, _ := json.Marshal(p.Blocks)
			_, e = tx.ExecContext(r.Context(), `INSERT INTO manga_pages(volume_id,page_index,image_key,image_type,size_bytes,width,height,blocks,original_blocks) VALUES($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$8::jsonb) ON CONFLICT(volume_id,page_index) DO NOTHING`, v.ID, n, key, mime, len(data), p.Width, p.Height, string(blocks))
			if e == nil {
				var saved string
				e = tx.QueryRowContext(r.Context(), `SELECT image_key FROM manga_pages WHERE volume_id=$1 AND page_index=$2`, v.ID, n).Scan(&saved)
				if e == nil && saved != key {
					e = fmt.Errorf("page exists")
				}
			}
		}
		if e == nil {
			e = tx.Commit()
		}
	}
	if e != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := h.mangaStore.Delete(ctx, key); err != nil {
			_, _ = h.db.ExecContext(ctx, `INSERT INTO manga_image_gc(image_key) VALUES($1) ON CONFLICT DO NOTHING`, key)
		}
		WriteError(w, 409, "Trang đã tồn tại hoặc phiên nhập đã thay đổi; tải lại để tiếp tục")
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) editMangaOCR(w http.ResponseWriter, r *http.Request, v mangaVolume, n int) {
	var in struct {
		Revision int        `json:"revision"`
		Lines    [][]string `json:"lines"`
		Restore  bool       `json:"restore"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in) != nil || in.Revision < 1 {
		WriteError(w, 422, "Dữ liệu sửa OCR không hợp lệ")
		return
	}
	tx, e := h.db.BeginTx(r.Context(), nil)
	if e != nil {
		writeServiceError(w, r, e)
		return
	}
	defer tx.Rollback()
	var p manga.Page
	var raw, original []byte
	var revision int
	e = tx.QueryRowContext(r.Context(), `SELECT p.width,p.height,p.blocks,p.original_blocks,p.revision FROM manga_pages p JOIN manga_volumes v ON v.id=p.volume_id WHERE p.volume_id=$1 AND p.page_index=$2 AND v.owner_user_id=$3 FOR UPDATE OF p`, v.ID, n, v.OwnerID).Scan(&p.Width, &p.Height, &raw, &original, &revision)
	if e == sql.ErrNoRows {
		WriteError(w, 404, "Không có trang")
		return
	}
	if e != nil {
		writeServiceError(w, r, e)
		return
	}
	if revision != in.Revision {
		WriteError(w, 409, "Trang đã được sửa ở phiên khác. Tải lại trang trước khi lưu")
		return
	}
	if json.Unmarshal(raw, &p.Blocks) != nil {
		WriteError(w, 500, "OCR bị lỗi")
		return
	}
	if in.Restore {
		if json.Unmarshal(original, &p.Blocks) != nil {
			WriteError(w, 500, "OCR gốc bị lỗi")
			return
		}
	} else {
		if len(in.Lines) != len(p.Blocks) {
			WriteError(w, 422, "Chỉ được sửa chữ, không được đổi khối OCR")
			return
		}
		for i := range p.Blocks {
			p.Blocks[i].Lines = in.Lines[i]
		}
	}
	if e = manga.Validate(&p); e != nil {
		WriteError(w, 422, e.Error())
		return
	}
	next, _ := json.Marshal(p.Blocks)
	_, e = tx.ExecContext(r.Context(), `INSERT INTO manga_ocr_history(volume_id,page_index,revision,blocks) VALUES($1,$2,$3,$4::jsonb)`, v.ID, n, revision, string(raw))
	if e == nil {
		_, e = tx.ExecContext(r.Context(), `UPDATE manga_pages SET blocks=$3::jsonb,revision=revision+1,updated_at=NOW() WHERE volume_id=$1 AND page_index=$2`, v.ID, n, string(next))
	}
	if e == nil {
		_, e = tx.ExecContext(r.Context(), `DELETE FROM manga_ocr_history WHERE volume_id=$1 AND page_index=$2 AND revision<$3`, v.ID, n, revision-19)
	}
	if e == nil {
		e = tx.Commit()
	}
	if e != nil {
		writeServiceError(w, r, e)
		return
	}
	WriteJSON(w, 200, map[string]any{"page": p, "revision": revision + 1})
}
func (h *Handler) RunMangaCleanup(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rows, e := h.db.QueryContext(ctx, `SELECT image_key FROM manga_image_gc ORDER BY created_at LIMIT 20`)
			if e != nil {
				continue
			}
			keys := []string{}
			for rows.Next() {
				var key string
				if rows.Scan(&key) == nil {
					keys = append(keys, key)
				}
			}
			rows.Close()
			for _, key := range keys {
				op, cancel := context.WithTimeout(ctx, 15*time.Second)
				e = h.mangaStore.Delete(op, key)
				cancel()
				if e == nil {
					_, _ = h.db.ExecContext(ctx, `DELETE FROM manga_image_gc WHERE image_key=$1`, key)
				} else {
					log.Printf("manga image cleanup pending")
				}
			}
		}
	}
}
func (h *Handler) exportMangaHTML(w http.ResponseWriter, r *http.Request, v mangaVolume) {
	var source, name string
	if e := h.db.QueryRowContext(r.Context(), `SELECT html_source,source_name FROM manga_volumes WHERE id=$1`, v.ID).Scan(&source, &name); e != nil {
		writeServiceError(w, r, e)
		return
	}
	if source == "" {
		WriteError(w, 422, "Tập truyện này được nhập từ Mokuro JSON; hãy nhập HTML để xuất lại đúng file gốc")
		return
	}
	rows, e := h.db.QueryContext(r.Context(), `SELECT page_index,width,height,blocks FROM manga_pages WHERE volume_id=$1 ORDER BY page_index`, v.ID)
	if e != nil {
		writeServiceError(w, r, e)
		return
	}
	defer rows.Close()
	pages := []manga.Page{}
	for rows.Next() {
		var p manga.Page
		var index int
		var raw []byte
		if e = rows.Scan(&index, &p.Width, &p.Height, &raw); e != nil {
			writeServiceError(w, r, e)
			return
		}
		if index != len(pages) || json.Unmarshal(raw, &p.Blocks) != nil {
			WriteError(w, 409, "Các trang chưa đầy đủ")
			return
		}
		pages = append(pages, p)
	}
	if e = rows.Err(); e != nil {
		writeServiceError(w, r, e)
		return
	}
	raw, e := manga.ExportHTML(source, pages)
	if e != nil {
		WriteError(w, 422, e.Error())
		return
	}
	if name == "" {
		name = "truyen.html"
	}
	name = strings.TrimSuffix(name, ".html") + "-edited.html"
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
	w.Write(raw)
}
