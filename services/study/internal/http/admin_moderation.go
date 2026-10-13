package http

import (
	"encoding/json"
	"fmt"
	"github.com/hunguyen1324/hquizlet-platform/packages/authz"
	"net/http"
	"strconv"
)

func (h *Handler) adminModeration(w http.ResponseWriter, r *http.Request) {
	p := principalFromRequest(r)
	if !authz.Can(p, authz.ActionDashboardViewAdmin, authz.Resource{Type: "platform"}) {
		WriteError(w, 403, "admin access required")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		WriteError(w, 422, "invalid ID")
		return
	}
	resource := fmt.Sprintf("hq:study-set/%d", id)
	if r.Method == "GET" {
		var payload []byte
		err = h.db.QueryRowContext(r.Context(), `SELECT jsonb_build_object('studySet',to_jsonb(s),'flashcards',COALESCE((SELECT jsonb_agg(to_jsonb(f)) FROM flashcards f WHERE f.study_set_id=s.id),'[]'::jsonb),'questions',COALESCE((SELECT jsonb_agg(to_jsonb(q)) FROM quiz_question q WHERE q.study_set_id=s.id),'[]'::jsonb)) FROM study_sets s WHERE s.id=$1`, id).Scan(&payload)
		if err != nil {
			WriteError(w, 404, "content not found")
			return
		}
		_, err = h.db.ExecContext(r.Context(), `INSERT INTO audit_log(actor_id,action,resource,result,request_id) VALUES($1,'study-set:Read',$2,'allow',$3)`, p.UserID, resource, r.Header.Get("X-Request-ID"))
		if err != nil {
			WriteError(w, 503, "audit unavailable")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(payload)
		return
	}
	var in struct {
		Hidden bool   `json:"hidden"`
		Reason string `json:"reason"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&in) != nil || in.Reason == "" {
		WriteError(w, 422, "moderation reason required")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		WriteError(w, 503, "database unavailable")
		return
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(r.Context(), `UPDATE study_sets SET moderation_hidden=$2 WHERE id=$1`, id, in.Hidden)
	if err != nil {
		WriteError(w, 500, "moderation failed")
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		WriteError(w, 404, "content not found")
		return
	}
	detail, _ := json.Marshal(in)
	_, err = tx.ExecContext(r.Context(), `INSERT INTO audit_log(actor_id,action,resource,result,request_id,detail) VALUES($1,'study-set:Publish',$2,'allow',$3,$4::jsonb)`, p.UserID, resource, r.Header.Get("X-Request-ID"), detail)
	if err != nil || tx.Commit() != nil {
		WriteError(w, 503, "moderation audit failed")
		return
	}
	WriteJSON(w, 200, map[string]any{"id": id, "hidden": in.Hidden})
}
