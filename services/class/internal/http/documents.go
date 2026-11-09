package http

import (
	"bytes"
	"database/sql"
	"io"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	classservice "github.com/hunguyen1324/hquizlet-platform/services/class/internal/service"
)

const maxDocumentBytes = 20 * 1024 * 1024

type classDocument struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	ContentType string    `json:"contentType"`
	SizeBytes   int64     `json:"sizeBytes"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Documents are private class attachments, never public asset URLs.
func (h *Handler) documentRouter(w http.ResponseWriter, r *http.Request, classID int64, parts []string) {
	w.Header().Set("Cache-Control", "private, no-store")
	uid := userIDFromHeader(r)
	cls, err := h.classes.GetByID(r.Context(), classID, uid)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	role, err := h.members.GetRole(r.Context(), classID, uid)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	staff := cls.OwnerUserID == uid || role == "owner" || role == "teacher"
	if !staff && role == "" {
		writeServiceError(w, r, classservice.ErrForbidden)
		return
	}
	if len(parts) > 1 {
		WriteError(w, 404, "not found")
		return
	}
	var id int64
	if len(parts) == 1 {
		id, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil || id <= 0 {
			WriteError(w, 400, "invalid document id")
			return
		}
	}
	if r.Method == http.MethodGet && id == 0 {
		rows, err := h.db.QueryContext(r.Context(), `SELECT id,name,content_type,octet_length(data),created_at FROM class_documents WHERE class_id=$1 ORDER BY created_at DESC,id DESC`, classID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		defer rows.Close()
		docs := []classDocument{}
		for rows.Next() {
			var d classDocument
			if err := rows.Scan(&d.ID, &d.Name, &d.ContentType, &d.SizeBytes, &d.CreatedAt); err != nil {
				writeServiceError(w, r, err)
				return
			}
			docs = append(docs, d)
		}
		if err := rows.Err(); err != nil {
			writeServiceError(w, r, err)
			return
		}
		WriteJSON(w, 200, docs)
		return
	}
	if r.Method == http.MethodGet && id > 0 {
		var name, contentType string
		var data []byte
		err := h.db.QueryRowContext(r.Context(), `SELECT name,content_type,data FROM class_documents WHERE class_id=$1 AND id=$2`, classID, id).Scan(&name, &contentType, &data)
		if err == sql.ErrNoRows {
			WriteError(w, 404, "document not found")
			return
		}
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Write(data)
		return
	}
	if !staff {
		writeServiceError(w, r, classservice.ErrForbidden)
		return
	}
	if r.Method == http.MethodPost && id == 0 {
		name := strings.TrimSpace(r.URL.Query().Get("name"))
		if name == "" || len([]rune(name)) > 255 || strings.ContainsAny(name, "/\\\r\n\x00") {
			WriteError(w, 422, "Tên tài liệu không hợp lệ")
			return
		}
		contentType := documentContentType(name)
		if contentType == "" {
			WriteError(w, 422, "Chỉ hỗ trợ PDF, Word, PowerPoint và Excel")
			return
		}
		data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxDocumentBytes))
		if err != nil {
			WriteError(w, 413, "Tài liệu tối đa 20 MiB")
			return
		}
		if !validDocumentData(name, data) {
			WriteError(w, 422, "Nội dung file không phù hợp với định dạng tài liệu")
			return
		}
		d := classDocument{Name: name, ContentType: contentType, SizeBytes: int64(len(data))}
		err = h.db.QueryRowContext(r.Context(), `INSERT INTO class_documents(class_id,name,content_type,data) VALUES($1,$2,$3,$4) RETURNING id,created_at`, classID, name, contentType, data).Scan(&d.ID, &d.CreatedAt)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		WriteJSON(w, 201, d)
		return
	}
	if r.Method == http.MethodDelete && id > 0 {
		result, err := h.db.ExecContext(r.Context(), `DELETE FROM class_documents WHERE class_id=$1 AND id=$2`, classID, id)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		n, err := result.RowsAffected()
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		if n == 0 {
			WriteError(w, 404, "document not found")
			return
		}
		w.WriteHeader(204)
		return
	}
	WriteError(w, 405, "method not allowed")
}

func documentContentType(name string) string {
	return map[string]string{
		".pdf": "application/pdf", ".doc": "application/msword", ".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".ppt": "application/vnd.ms-powerpoint", ".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		".xls": "application/vnd.ms-excel", ".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	}[strings.ToLower(path.Ext(name))]
}

func validDocumentData(name string, data []byte) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".pdf":
		return bytes.HasPrefix(data, []byte("%PDF-"))
	case ".docx", ".pptx", ".xlsx":
		return bytes.HasPrefix(data, []byte{'P', 'K', 3, 4})
	case ".doc", ".ppt", ".xls":
		return bytes.HasPrefix(data, []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1})
	}
	return false
}
