package middleware

import (
	"database/sql"
	"github.com/hunguyen1324/hquizlet-platform/packages/authz"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/service"
	"net/http"
	"strconv"
	"strings"
)

// StudyWritePolicy enforces managed-role policy in addition to resource ownership checks.
func StudyWritePolicy(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" || r.Method == "HEAD" || (!strings.HasPrefix(r.URL.Path, "/v1/study-sets") && !strings.HasPrefix(r.URL.Path, "/v1/flashcards/")) {
				next.ServeHTTP(w, r)
				return
			}
			uid, _ := strconv.ParseInt(r.Header.Get("X-User-ID"), 10, 64)
			p := authz.Principal{UserID: uid, LegacyRole: r.Header.Get("X-User-Role"), PlatformRoles: authz.ParsePlatformRoles(r.Header.Get("X-User-Roles"))}
			action := authz.ActionStudySetUpdate
			res := authz.Resource{Type: "study-set", OwnerUserID: uid}
			if r.URL.Path == "/v1/study-sets" {
				action = authz.ActionStudySetCreate
			} else {
				prefix := "/v1/study-sets/"
				if strings.HasPrefix(r.URL.Path, "/v1/flashcards/") {
					prefix = "/v1/flashcards/"
				}
				parts := strings.Split(strings.TrimPrefix(r.URL.Path, prefix), "/")
				if len(parts) > 1 && (parts[1] == "progress" || parts[1] == "play") && !(len(parts) == 3 && parts[2] == "audio") {
					next.ServeHTTP(w, r)
					return
				}
				id, err := strconv.ParseInt(parts[0], 10, 64)
				if err != nil || id < 1 {
					http.Error(w, "invalid resource", 422)
					return
				}
				if len(parts) == 1 && prefix == "/v1/study-sets/" && r.Method == "DELETE" {
					action = authz.ActionStudySetDelete
				}
				if len(parts) > 1 && parts[1] == "import" {
					action = authz.ActionStudySetImport
				}
				query := `SELECT id,user_id FROM study_sets WHERE id=$1 AND NOT moderation_hidden`
				if prefix == "/v1/flashcards/" {
					query = `SELECT s.id,s.user_id FROM flashcards f JOIN study_sets s ON s.id=f.study_set_id WHERE f.id=$1 AND NOT s.moderation_hidden`
				}
				if err = db.QueryRowContext(r.Context(), query, id).Scan(&res.ID, &res.OwnerUserID); err != nil {
					http.Error(w, "resource unavailable", 404)
					return
				}
			}
			if !authz.Can(p, action, res) {
				http.Error(w, "study permission required", 403)
				return
			}
			r = r.WithContext(service.WithStudyWritePermission(r.Context(), uid, res.ID))
			next.ServeHTTP(w, r)
		})
	}
}
