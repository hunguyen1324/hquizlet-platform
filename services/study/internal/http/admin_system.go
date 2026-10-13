package http

import (
	"github.com/hunguyen1324/hquizlet-platform/packages/authz"
	"net/http"
)

func (h *Handler) adminSystem(w http.ResponseWriter, r *http.Request) {
	if !authz.Can(principalFromRequest(r), authz.ActionDashboardViewAdmin, authz.Resource{Type: "platform"}) {
		WriteError(w, 403, "forbidden")
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `SELECT status,count(*) FROM import_jobs GROUP BY status`)
	if err != nil {
		WriteError(w, 503, "import queue unavailable")
		return
	}
	counts := map[string]int{"pending": 0, "running": 0, "done": 0, "failed": 0}
	for rows.Next() {
		var status string
		var count int
		if rows.Scan(&status, &count) != nil {
			rows.Close()
			WriteError(w, 503, "import queue unavailable")
			return
		}
		counts[status] = count
	}
	scanErr := rows.Err()
	rows.Close()
	if scanErr != nil {
		WriteError(w, 503, "import queue unavailable")
		return
	}
	failed, err := h.db.QueryContext(r.Context(), `SELECT id,error_msg,updated_at FROM import_jobs WHERE status='failed' ORDER BY updated_at DESC LIMIT 10`)
	if err != nil {
		WriteError(w, 503, "import errors unavailable")
		return
	}
	defer failed.Close()
	errors := []map[string]any{}
	for failed.Next() {
		var id int64
		var message string
		var at any
		if failed.Scan(&id, &message, &at) != nil {
			WriteError(w, 503, "import errors unavailable")
			return
		}
		errors = append(errors, map[string]any{"id": id, "message": message, "at": at})
	}
	if failed.Err() != nil {
		WriteError(w, 503, "import errors unavailable")
		return
	}
	WriteJSON(w, 200, map[string]any{"importQueue": counts, "recentErrors": errors})
}

func (h *Handler) adminTopContent(w http.ResponseWriter, r *http.Request) {
	if !authz.Can(principalFromRequest(r), authz.ActionDashboardViewAdmin, authz.Resource{Type: "platform"}) {
		WriteError(w, 403, "forbidden")
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `SELECT s.id,s.title,count(l.id),COALESCE(round(avg(CASE WHEN l.total>0 THEN 100.0*l.score/l.total END)),0) FROM study_sets s LEFT JOIN learning_sessions l ON l.study_set_id=s.id AND l.created_at>=now()-interval '28 days' WHERE NOT s.moderation_hidden GROUP BY s.id,s.title ORDER BY count(l.id) DESC,s.id DESC LIMIT 10`)
	if err != nil {
		WriteError(w, 503, "content statistics unavailable")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, sessions, score int64
		var title string
		if rows.Scan(&id, &title, &sessions, &score) != nil {
			WriteError(w, 503, "content statistics unavailable")
			return
		}
		items = append(items, map[string]any{"id": id, "title": title, "sessions": sessions, "avgScorePct": score})
	}
	if rows.Err() != nil {
		WriteError(w, 503, "content statistics unavailable")
		return
	}
	WriteJSON(w, 200, map[string]any{"items": items})
}
