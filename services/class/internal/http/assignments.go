package http

import (
	"database/sql"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type assignmentFile struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
	SizeBytes   int64  `json:"sizeBytes"`
}
type classAssignment struct {
	ID           int64            `json:"id"`
	Title        string           `json:"title"`
	Instructions string           `json:"instructions"`
	DueAt        *time.Time       `json:"dueAt"`
	Status       string           `json:"status"`
	CreatedBy    int64            `json:"createdBy"`
	CreatedAt    time.Time        `json:"createdAt"`
	UpdatedAt    time.Time        `json:"updatedAt"`
	Files        []assignmentFile `json:"files"`
}
type assignmentInput struct {
	Title         string     `json:"title"`
	Instructions  string     `json:"instructions"`
	DueAt         *time.Time `json:"dueAt"`
	Status        string     `json:"status"`
	DeleteFileIDs []int64    `json:"deleteFileIds"`
}

func (h *Handler) hasClassAction(r *http.Request, classID int64, action string) bool {
	return h.access != nil && h.access.Require(r.Context(), classID, userIDFromHeader(r), action) == nil
}
func (h *Handler) assignmentRouter(w http.ResponseWriter, r *http.Request, classID int64, parts []string) {
	w.Header().Set("Cache-Control", "private, no-store")
	if h.access == nil {
		WriteError(w, 503, "assignments unavailable")
		return
	}
	actions, err := h.access.Effective(r.Context(), classID, userIDFromHeader(r))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	has := func(action string) bool {
		for _, a := range actions {
			if a == action {
				return true
			}
		}
		return false
	}
	canDraft := has("class-assignment:Update") || has("class-assignment:Publish") || has("class-assignment:Delete")
	uid := userIDFromHeader(r)
	if len(parts) == 0 && r.Method == http.MethodGet {
		if !has("class-assignment:Read") && !canDraft && !has("class-assignment:Create") {
			WriteError(w, 403, "Không có quyền xem bài tập")
			return
		}
		rows, e := h.db.QueryContext(r.Context(), `SELECT a.id,a.title,a.instructions,a.due_at,a.status,a.created_by,a.created_at,a.updated_at,COALESCE((SELECT jsonb_agg(jsonb_build_object('id',f.id,'name',f.name,'contentType',f.content_type,'sizeBytes',octet_length(f.data)) ORDER BY f.id) FROM class_assignment_files f WHERE f.assignment_id=a.id),'[]'::jsonb) FROM class_assignments a WHERE a.class_id=$1 AND ((a.status='published' AND $2) OR $3 OR (a.created_by=$4 AND $5)) ORDER BY a.created_at DESC,a.id DESC`, classID, has("class-assignment:Read"), canDraft, uid, has("class-assignment:Create"))
		if e != nil {
			writeServiceError(w, r, e)
			return
		}
		defer rows.Close()
		out := []classAssignment{}
		for rows.Next() {
			var a classAssignment
			var raw []byte
			if e = rows.Scan(&a.ID, &a.Title, &a.Instructions, &a.DueAt, &a.Status, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt, &raw); e != nil {
				writeServiceError(w, r, e)
				return
			}
			if e = json.Unmarshal(raw, &a.Files); e != nil {
				writeServiceError(w, r, e)
				return
			}
			out = append(out, a)
		}
		if e = rows.Err(); e != nil {
			writeServiceError(w, r, e)
			return
		}
		WriteJSON(w, 200, out)
		return
	}
	var id int64
	if len(parts) > 0 {
		id, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil || id <= 0 {
			WriteError(w, 400, "invalid assignment")
			return
		}
	}
	if len(parts) == 3 && parts[1] == "files" && r.Method == http.MethodGet {
		fileID, e := strconv.ParseInt(parts[2], 10, 64)
		if e != nil || fileID <= 0 {
			WriteError(w, 400, "invalid file")
			return
		}
		var f assignmentFile
		var data []byte
		e = h.db.QueryRowContext(r.Context(), `SELECT f.name,f.content_type,f.data FROM class_assignment_files f JOIN class_assignments a ON a.id=f.assignment_id WHERE a.class_id=$1 AND a.id=$2 AND f.id=$3 AND ((a.status='published' AND $4) OR $5 OR (a.created_by=$6 AND $7))`, classID, id, fileID, has("class-assignment:Read"), canDraft, uid, has("class-assignment:Create")).Scan(&f.Name, &f.ContentType, &data)
		if e == sql.ErrNoRows {
			WriteError(w, 404, "file not found")
			return
		}
		if e != nil {
			writeServiceError(w, r, e)
			return
		}
		w.Header().Set("Content-Type", f.ContentType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": f.Name}))
		w.Write(data)
		return
	}
	if len(parts) > 1 {
		WriteError(w, 404, "not found")
		return
	}
	if r.Method == http.MethodDelete && id > 0 {
		if !h.requireClassAction(w, r, classID, "class-assignment:Delete") {
			return
		}
		result, e := h.db.ExecContext(r.Context(), `DELETE FROM class_assignments WHERE class_id=$1 AND id=$2`, classID, id)
		if e != nil {
			writeServiceError(w, r, e)
			return
		}
		n, _ := result.RowsAffected()
		if n == 0 {
			WriteError(w, 404, "assignment not found")
			return
		}
		w.WriteHeader(204)
		return
	}
	if (r.Method != http.MethodPost || id != 0) && (r.Method != http.MethodPut || id == 0) {
		WriteError(w, 405, "method not allowed")
		return
	}
	action := "class-assignment:Create"
	if id > 0 {
		action = "class-assignment:Update"
	}
	if !h.requireClassAction(w, r, classID, action) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 40*1024*1024+1024*1024)
	if err = r.ParseMultipartForm(1024 * 1024); err != nil {
		if r.MultipartForm != nil {
			r.MultipartForm.RemoveAll()
		}
		WriteError(w, 413, "Tệp đính kèm tối đa 40 MiB mỗi bài")
		return
	}
	defer r.MultipartForm.RemoveAll()
	var in assignmentInput
	if json.Unmarshal([]byte(r.FormValue("metadata")), &in) != nil {
		WriteError(w, 422, "Thông tin bài tập không hợp lệ")
		return
	}
	in.Title = strings.TrimSpace(in.Title)
	in.Instructions = strings.TrimSpace(in.Instructions)
	if in.Title == "" || len([]rune(in.Title)) > 200 || len([]rune(in.Instructions)) > 20000 || (in.Status != "draft" && in.Status != "published") || len(in.DeleteFileIDs) > 5 {
		WriteError(w, 422, "Kiểm tra tiêu đề, nội dung và trạng thái bài tập")
		return
	}
	files := r.MultipartForm.File["files"]
	if len(files) > 5 {
		WriteError(w, 422, "Tối đa 5 file mỗi bài tập")
		return
	}
	type upload struct {
		name, ct string
		data     []byte
	}
	uploads := []upload{}
	for _, f := range files {
		name := strings.TrimSpace(f.Filename)
		ct := documentContentType(name)
		if name == "" || len([]rune(name)) > 255 || strings.ContainsAny(name, "/\\\r\n\x00") || ct == "" {
			WriteError(w, 422, "Chỉ nhận file PDF, Word, Excel, PowerPoint với tên hợp lệ")
			return
		}
		stream, e := f.Open()
		if e != nil {
			writeServiceError(w, r, e)
			return
		}
		data, e := io.ReadAll(io.LimitReader(stream, maxDocumentBytes+1))
		stream.Close()
		if e != nil || len(data) > maxDocumentBytes {
			WriteError(w, 413, "Mỗi file tối đa 20 MiB")
			return
		}
		if !validDocumentData(name, data) {
			WriteError(w, 422, "Nội dung file không khớp định dạng")
			return
		}
		uploads = append(uploads, upload{name, ct, data})
	}
	tx, e := h.db.BeginTx(r.Context(), nil)
	if e != nil {
		writeServiceError(w, r, e)
		return
	}
	defer tx.Rollback()
	a := classAssignment{ID: id, Title: in.Title, Instructions: in.Instructions, DueAt: in.DueAt, Status: in.Status, CreatedBy: uid, Files: []assignmentFile{}}
	oldStatus := "draft"
	if id > 0 {
		e = tx.QueryRowContext(r.Context(), `SELECT status,created_by,created_at FROM class_assignments WHERE class_id=$1 AND id=$2 FOR UPDATE`, classID, id).Scan(&oldStatus, &a.CreatedBy, &a.CreatedAt)
		if e == sql.ErrNoRows {
			WriteError(w, 404, "assignment not found")
			return
		}
		if e != nil {
			writeServiceError(w, r, e)
			return
		}
	}
	if (in.Status == "published" || oldStatus != in.Status) && !has("class-assignment:Publish") {
		WriteError(w, 403, "Bạn chưa có quyền giao hoặc cập nhật bài đã giao")
		return
	}
	if id == 0 {
		e = tx.QueryRowContext(r.Context(), `INSERT INTO class_assignments(class_id,title,instructions,due_at,status,created_by) VALUES($1,$2,$3,$4,$5,$6) RETURNING id,created_at,updated_at`, classID, in.Title, in.Instructions, in.DueAt, in.Status, uid).Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt)
	} else {
		e = tx.QueryRowContext(r.Context(), `UPDATE class_assignments SET title=$3,instructions=$4,due_at=$5,status=$6,updated_at=NOW() WHERE class_id=$1 AND id=$2 RETURNING updated_at`, classID, id, in.Title, in.Instructions, in.DueAt, in.Status).Scan(&a.UpdatedAt)
	}
	if e != nil {
		writeServiceError(w, r, e)
		return
	}
	for _, fileID := range in.DeleteFileIDs {
		result, e := tx.ExecContext(r.Context(), `DELETE FROM class_assignment_files WHERE assignment_id=$1 AND id=$2`, a.ID, fileID)
		if e != nil {
			writeServiceError(w, r, e)
			return
		}
		n, _ := result.RowsAffected()
		if n == 0 {
			WriteError(w, 422, "File cần xóa không thuộc bài tập")
			return
		}
	}
	var count, total int64
	if e = tx.QueryRowContext(r.Context(), `SELECT count(*),COALESCE(sum(octet_length(data)),0) FROM class_assignment_files WHERE assignment_id=$1`, a.ID).Scan(&count, &total); e != nil {
		writeServiceError(w, r, e)
		return
	}
	for _, f := range uploads {
		count++
		total += int64(len(f.data))
	}
	if count > 5 || total > 40*1024*1024 {
		WriteError(w, 422, "Mỗi bài tối đa 5 file và tổng 40 MiB")
		return
	}
	for _, f := range uploads {
		_, e = tx.ExecContext(r.Context(), `INSERT INTO class_assignment_files(assignment_id,name,content_type,data) VALUES($1,$2,$3,$4)`, a.ID, f.name, f.ct, f.data)
		if e != nil {
			writeServiceError(w, r, e)
			return
		}
	}
	rows, e := tx.QueryContext(r.Context(), `SELECT id,name,content_type,octet_length(data) FROM class_assignment_files WHERE assignment_id=$1 ORDER BY id`, a.ID)
	if e != nil {
		writeServiceError(w, r, e)
		return
	}
	for rows.Next() {
		var f assignmentFile
		if e = rows.Scan(&f.ID, &f.Name, &f.ContentType, &f.SizeBytes); e != nil {
			rows.Close()
			writeServiceError(w, r, e)
			return
		}
		a.Files = append(a.Files, f)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		writeServiceError(w, r, e)
		return
	}
	if e = tx.Commit(); e != nil {
		writeServiceError(w, r, e)
		return
	}
	status := 200
	if id == 0 {
		status = 201
	}
	WriteJSON(w, status, a)
}
