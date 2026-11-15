package http

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	classservice "github.com/hunguyen1324/hquizlet-platform/services/class/internal/service"
)

func (h *Handler) requireClassAction(w http.ResponseWriter, r *http.Request, classID int64, action string) bool {
	if h.access == nil {
		return true
	} // Legacy test/service construction keeps its existing authorization.
	if err := h.access.Require(r.Context(), classID, userIDFromHeader(r), action); err != nil {
		writeServiceError(w, r, err)
		return false
	}
	return true
}
func randomRoleKey() string {
	var b [16]byte
	_, err := rand.Read(b[:])
	if err != nil {
		panic(err)
	}
	return "custom-" + hex.EncodeToString(b[:])
}
func validateRole(role *classservice.ClassRole) bool {
	role.Name = strings.TrimSpace(role.Name)
	role.Description = strings.TrimSpace(role.Description)
	if role.Name == "" || len([]rune(role.Name)) > 80 || len([]rune(role.Description)) > 500 || len(role.Allow) > len(classservice.ClassActions) || len(role.Deny) > len(classservice.ClassActions) {
		return false
	}
	for _, list := range [][]string{role.Allow, role.Deny} {
		for _, a := range list {
			if !classservice.KnownClassAction(a) {
				return false
			}
		}
	}
	if role.Allow == nil {
		role.Allow = []string{}
	}
	if role.Deny == nil {
		role.Deny = []string{}
	}
	return true
}
func (h *Handler) accessRouter(w http.ResponseWriter, r *http.Request, classID int64, parts []string) {
	w.Header().Set("Cache-Control", "private, no-store")
	if h.access == nil {
		WriteError(w, 503, "class permissions unavailable")
		return
	}
	uid := userIDFromHeader(r)
	if len(parts) == 1 && parts[0] == "me" && r.Method == http.MethodGet {
		actions, err := h.access.Effective(r.Context(), classID, uid)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		WriteJSON(w, 200, map[string]any{"actions": actions})
		return
	}
	if !h.requireClassAction(w, r, classID, "class-access:Manage") {
		return
	}
	if len(parts) == 0 && r.Method == http.MethodGet {
		roles, err := h.access.Roles(r.Context(), classID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		rows, err := h.db.QueryContext(r.Context(), `SELECT m.user_id,m.role,COALESCE(a.role_keys,'null'::jsonb) FROM class_members m LEFT JOIN class_member_access a ON a.class_id=m.class_id AND a.user_id=m.user_id WHERE m.class_id=$1 ORDER BY m.joined_at,m.user_id`, classID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		defer rows.Close()
		members := []map[string]any{}
		for rows.Next() {
			var id int64
			var legacy string
			var raw []byte
			if err = rows.Scan(&id, &legacy, &raw); err != nil {
				writeServiceError(w, r, err)
				return
			}
			keys := []string{}
			if string(raw) == "null" {
				keys = []string{legacy}
			} else if err = json.Unmarshal(raw, &keys); err != nil {
				writeServiceError(w, r, err)
				return
			}
			members = append(members, map[string]any{"userId": id, "legacyRole": legacy, "roleKeys": keys})
		}
		if err = rows.Err(); err != nil {
			writeServiceError(w, r, err)
			return
		}
		if h.users != nil {
			ids := []int64{}
			for _, m := range members {
				ids = append(ids, m["userId"].(int64))
			}
			if users, e := h.users.Lookup(r.Context(), ids); e == nil {
				for _, m := range members {
					u := users[m["userId"].(int64)]
					m["name"] = u.Name
					m["email"] = u.Email
				}
			}
		}
		WriteJSON(w, 200, map[string]any{"roles": roles, "members": members, "catalog": classservice.ClassActions})
		return
	}
	// Serialize role edits and assignments within a class, keeping role references consistent.
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	defer tx.Rollback()
	var owner int64
	if err = tx.QueryRowContext(r.Context(), `SELECT owner_user_id FROM classes WHERE id=$1 FOR UPDATE`, classID).Scan(&owner); err != nil {
		writeServiceError(w, r, err)
		return
	}
	if len(parts) >= 1 && parts[0] == "roles" {
		key := ""
		if len(parts) == 2 {
			key = parts[1]
		} else if len(parts) != 1 {
			WriteError(w, 404, "not found")
			return
		}
		if key != "" && !strings.HasPrefix(key, "custom-") {
			WriteError(w, 422, "Role hệ thống không thể sửa hoặc xóa")
			return
		}
		if (r.Method == http.MethodPost && key == "") || (r.Method == http.MethodPut && key != "") {
			var role classservice.ClassRole
			if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&role) != nil || !validateRole(&role) {
				WriteError(w, 422, "Tên role hoặc danh sách quyền không hợp lệ")
				return
			}
			allow, _ := json.Marshal(role.Allow)
			deny, _ := json.Marshal(role.Deny)
			if key == "" {
				var count int
				if err = tx.QueryRowContext(r.Context(), `SELECT count(*) FROM class_role_definitions WHERE class_id=$1`, classID).Scan(&count); err != nil {
					writeServiceError(w, r, err)
					return
				}
				if count >= 50 {
					WriteError(w, 422, "Mỗi lớp tối đa 50 role tùy chỉnh")
					return
				}
				key = randomRoleKey()
				_, err = tx.ExecContext(r.Context(), `INSERT INTO class_role_definitions(class_id,key,name,description,allow_actions,deny_actions) VALUES($1,$2,$3,$4,$5::jsonb,$6::jsonb)`, classID, key, role.Name, role.Description, string(allow), string(deny))
			} else {
				var result sql.Result
				result, err = tx.ExecContext(r.Context(), `UPDATE class_role_definitions SET name=$3,description=$4,allow_actions=$5::jsonb,deny_actions=$6::jsonb WHERE class_id=$1 AND key=$2`, classID, key, role.Name, role.Description, string(allow), string(deny))
				if err == nil {
					n, _ := result.RowsAffected()
					if n == 0 {
						WriteError(w, 404, "role not found")
						return
					}
				}
			}
			if err != nil {
				writeServiceError(w, r, err)
				return
			}
			if err = tx.Commit(); err != nil {
				writeServiceError(w, r, err)
				return
			}
			role.Key = key
			role.Managed = false
			WriteJSON(w, 200, role)
			return
		}
		if r.Method == http.MethodDelete && key != "" {
			var used bool
			if err = tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM class_member_access WHERE class_id=$1 AND role_keys ? $2)`, classID, key).Scan(&used); err != nil {
				writeServiceError(w, r, err)
				return
			}
			if used {
				WriteError(w, 409, "Role đang được gán. Hãy thu hồi role trước khi xóa.")
				return
			}
			result, e := tx.ExecContext(r.Context(), `DELETE FROM class_role_definitions WHERE class_id=$1 AND key=$2`, classID, key)
			if e != nil {
				writeServiceError(w, r, e)
				return
			}
			n, _ := result.RowsAffected()
			if n == 0 {
				WriteError(w, 404, "role not found")
				return
			}
			if err = tx.Commit(); err != nil {
				writeServiceError(w, r, err)
				return
			}
			w.WriteHeader(204)
			return
		}
	}
	if len(parts) == 2 && parts[0] == "members" && r.Method == http.MethodPut {
		target, e := strconv.ParseInt(parts[1], 10, 64)
		if e != nil || target <= 0 {
			WriteError(w, 400, "invalid member")
			return
		}
		if target == owner {
			WriteError(w, 403, "Không thể thay đổi quyền chủ lớp")
			return
		}
		if target == uid {
			WriteError(w, 403, "Không thể tự thay đổi role của chính bạn")
			return
		}
		var in struct {
			RoleKeys []string `json:"roleKeys"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&in) != nil || len(in.RoleKeys) > 50 {
			WriteError(w, 422, "Danh sách role không hợp lệ")
			return
		}
		known := map[string]bool{}
		for _, r := range classservice.ManagedClassRoles() {
			if r.Key != "owner" {
				known[r.Key] = true
			}
		}
		rows, e := tx.QueryContext(r.Context(), `SELECT key FROM class_role_definitions WHERE class_id=$1`, classID)
		if e != nil {
			writeServiceError(w, r, e)
			return
		}
		for rows.Next() {
			var k string
			if e = rows.Scan(&k); e != nil {
				rows.Close()
				writeServiceError(w, r, e)
				return
			}
			known[k] = true
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			writeServiceError(w, r, e)
			return
		}
		keys := []string{}
		seen := map[string]bool{}
		for _, k := range in.RoleKeys {
			if !known[k] {
				WriteError(w, 422, "Role không thuộc lớp này")
				return
			}
			if !seen[k] {
				keys = append(keys, k)
				seen[k] = true
			}
		}
		var exists bool
		if err = tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM class_members WHERE class_id=$1 AND user_id=$2)`, classID, target).Scan(&exists); err != nil {
			writeServiceError(w, r, err)
			return
		}
		if !exists {
			WriteError(w, 404, "member not found")
			return
		}
		raw, _ := json.Marshal(keys)
		_, err = tx.ExecContext(r.Context(), `INSERT INTO class_member_access(class_id,user_id,role_keys) VALUES($1,$2,$3::jsonb) ON CONFLICT(class_id,user_id) DO UPDATE SET role_keys=EXCLUDED.role_keys`, classID, target, string(raw))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		if err = tx.Commit(); err != nil {
			writeServiceError(w, r, err)
			return
		}
		WriteJSON(w, 200, map[string]any{"roleKeys": keys})
		return
	}
	WriteError(w, 405, "method not allowed")
}
