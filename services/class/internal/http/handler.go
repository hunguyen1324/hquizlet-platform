// Package http contains HTTP handlers for the class service.
package http

import (
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/hunguyen1324/hquizlet-platform/packages/authz"
	"net/http"
	"strconv"
	"strings"

	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/model"
	classservice "github.com/hunguyen1324/hquizlet-platform/services/class/internal/service"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/videostorage"
)

// Handler holds all service dependencies for HTTP routing.
type Handler struct {
	classes       *classservice.ClassService
	members       *classservice.MemberService
	studySets     *classservice.ClassStudySetService
	activity      *classservice.ActivityService
	dashboard     *classservice.ClassDashboardService
	admin         *classservice.ClassAdminService
	db            *sql.DB
	videoStore    videostorage.Store
	videoSecret   []byte
	hlsSecret     []byte
	videoMaxBytes int64
}

// WithDashboard wires the class dashboard service (GET /v1/classes/{id}/dashboard).
func (h *Handler) WithDashboard(d *classservice.ClassDashboardService) *Handler {
	h.dashboard = d
	return h
}

// WithAdmin wires the platform-wide admin listing service (GET /v1/admin/classes).
func (h *Handler) WithAdmin(a *classservice.ClassAdminService) *Handler {
	h.admin = a
	return h
}

// ConfigureVideos enables private SFTP videos without changing public image storage.
func (h *Handler) ConfigureVideos(store videostorage.Store, secret string, maxBytes int64) {
	h.videoStore, h.videoSecret, h.videoMaxBytes = store, []byte(secret), maxBytes
}

// New creates a new Handler.
func New(
	classes *classservice.ClassService,
	members *classservice.MemberService,
	studySets *classservice.ClassStudySetService,
	activity *classservice.ActivityService,
	db *sql.DB,
) *Handler {
	return &Handler{
		classes:   classes,
		members:   members,
		studySets: studySets,
		activity:  activity,
		db:        db,
	}
}

// Register registers all routes on the given mux.
func (h *Handler) Register(mux *http.ServeMux) {
	// Health
	mux.HandleFunc("GET /healthz", h.health)

	// Class CRUD
	mux.HandleFunc("GET /v1/classes", h.listClasses)
	mux.HandleFunc("POST /v1/classes", h.createClass)
	mux.HandleFunc("/v1/classes/", h.classRouter)
	mux.HandleFunc("GET /v1/class-video-stream", h.streamPrivateVideo)
	mux.HandleFunc("GET /v1/class-video-hls", h.serveHLS)

	// Activity feed
	mux.HandleFunc("GET /v1/activity", h.getActivityFeed)

	// Admin console (ADM-06). The gateway already requires admin-console access; the
	// service re-checks dashboard:ViewAdmin through authz.Can before returning any row.
	mux.HandleFunc("GET /v1/admin/classes", h.adminListClasses)
	mux.HandleFunc("GET /v1/admin/class-stats", authz.DailyMetricsHandler(h.db, "class"))
}

// adminListClasses serves the platform-wide class list to admins only.
func (h *Handler) adminListClasses(w http.ResponseWriter, r *http.Request) {
	if h.admin == nil {
		WriteError(w, http.StatusServiceUnavailable, "admin listing unavailable")
		return
	}
	limit, offset := queryLimitOffset(r, 25)
	rows, total, err := h.admin.ListClasses(r.Context(), principalFromRequest(r), r.URL.Query().Get("search"), limit, offset)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"classes": rows, "total": total})
}

// queryLimitOffset parses ?limit/?offset with a bounded default.
func queryLimitOffset(r *http.Request, fallback int) (int, int) {
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit <= 0 || limit > 100 {
		limit = fallback
	}
	offset, err := strconv.Atoi(r.URL.Query().Get("offset"))
	if err != nil || offset < 0 {
		offset = 0
	}
	return limit, offset
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	body := map[string]string{"service": "class", "status": "ok", "database": "ok"}
	status := http.StatusOK
	if err := h.db.PingContext(r.Context()); err != nil {
		status = http.StatusServiceUnavailable
		body["status"] = "degraded"
		body["database"] = "offline"
	}
	WriteJSON(w, status, body)
}

// --- Class routes ---

func (h *Handler) listClasses(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromHeader(r)
	classes, err := h.classes.ListByUserID(r.Context(), userID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	if classes == nil {
		classes = []*model.ClassSummary{}
	}
	WriteJSON(w, http.StatusOK, classes)
}

func (h *Handler) createClass(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromHeader(r)
	var in model.CreateClassInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	class, err := h.classes.Create(r.Context(), principalFromRequest(r), in)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	// Record activity event
	_ = h.activity.RecordEvent(r.Context(), model.ActivityEvent{
		UserID:     userID,
		EventType:  "class.created",
		EntityType: "class",
		EntityID:   &class.ID,
		ClassID:    &class.ID,
		Metadata:   json.RawMessage(`{"className":"` + strings.ReplaceAll(class.Name, `"`, `\"`) + `"}`),
	})

	WriteJSON(w, http.StatusCreated, class)
}

func (h *Handler) classRouter(w http.ResponseWriter, r *http.Request) {
	parts := PathParts(r.URL.Path, "/v1/classes/")
	if len(parts) == 0 {
		WriteError(w, http.StatusNotFound, "not found")
		return
	}

	// Check if this is a join or invite-code/reset path
	if len(parts) == 2 && parts[1] == "join" {
		h.joinClass(w, r, parts[0])
		return
	}

	// Parse classID
	classID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid class id")
		return
	}

	// Sub-routes
	if len(parts) == 2 && parts[1] == "permissions" && r.Method == http.MethodGet {
		p := principalFromRequest(r)
		actions, err := h.classes.Permissions(r.Context(), p, classID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		WriteJSON(w, 200, map[string]any{"actions": actions, "platformRoles": p.PlatformRoles, "legacyRole": p.LegacyRole, "classId": classID})
		return
	}
	if len(parts) >= 2 && parts[1] == "videos" {
		h.videoRouter(w, r, classID, parts[2:])
		return
	}
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			h.getClassDetail(w, r, classID)
		case http.MethodPut:
			h.updateClass(w, r, classID)
		case http.MethodDelete:
			h.deleteClass(w, r, classID)
		default:
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// /v1/classes/{id}/invite-code/reset
	if len(parts) == 3 && parts[1] == "invite-code" && parts[2] == "reset" {
		if r.Method == http.MethodPost {
			h.resetInviteCode(w, r, classID)
		} else {
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// /v1/classes/{id}/dashboard (Phase 3)
	if len(parts) == 2 && parts[1] == "dashboard" {
		if r.Method == http.MethodGet {
			h.getClassDashboard(w, r, classID)
		} else {
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// /v1/classes/{id}/leaderboard (Phase 3, CLS-05)
	if len(parts) == 2 && parts[1] == "leaderboard" {
		if r.Method == http.MethodPut {
			h.setClassLeaderboard(w, r, classID)
		} else {
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// /v1/classes/{id}/members
	if len(parts) == 2 && parts[1] == "members" {
		switch r.Method {
		case http.MethodGet:
			h.listMembers(w, r, classID)
		case http.MethodPost:
			h.addMember(w, r, classID)
		default:
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// /v1/classes/{id}/members/me (leave)
	if len(parts) == 3 && parts[1] == "members" && parts[2] == "me" {
		if r.Method == http.MethodDelete {
			h.leaveClass(w, r, classID)
		} else {
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// /v1/classes/{id}/members/{userId}
	if len(parts) == 3 && parts[1] == "members" {
		targetUserID, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "invalid user id")
			return
		}
		switch r.Method {
		case http.MethodPut:
			h.updateMemberRole(w, r, classID, targetUserID)
		case http.MethodDelete:
			h.removeMember(w, r, classID, targetUserID)
		default:
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// /v1/classes/{id}/study-sets
	if len(parts) == 2 && parts[1] == "study-sets" {
		switch r.Method {
		case http.MethodGet:
			h.listClassStudySets(w, r, classID)
		case http.MethodPost:
			h.addStudySetToClass(w, r, classID)
		default:
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// /v1/classes/{id}/study-sets/{studySetId}
	if len(parts) == 3 && parts[1] == "study-sets" {
		if r.Method == http.MethodDelete {
			studySetID, err := strconv.ParseInt(parts[2], 10, 64)
			if err != nil {
				WriteError(w, http.StatusBadRequest, "invalid study set id")
				return
			}
			h.removeStudySetFromClass(w, r, classID, studySetID)
		} else {
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	WriteError(w, http.StatusNotFound, "not found")
}

// --- Class CRUD handlers ---

func (h *Handler) getClassDetail(w http.ResponseWriter, r *http.Request, classID int64) {
	userID := userIDFromHeader(r)
	class, err := h.classes.GetByID(r.Context(), classID, userID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	// Enrich with member/study set counts
	memberCount, _ := h.members.CountByClass(r.Context(), classID)
	studySetCount, _ := h.studySets.CountByClass(r.Context(), classID)

	detail := model.ClassDetail{
		ClassSummary: model.ClassSummary{
			ID:            class.ID,
			Name:          class.Name,
			Description:   class.Description,
			InviteCode:    class.InviteCode,
			MemberCount:   memberCount,
			StudySetCount: studySetCount,
			CreatedAt:     class.CreatedAt,
			UpdatedAt:     class.UpdatedAt,
		},
		MaxMembers: class.MaxMembers,
	}

	// Determine my role
	role, _ := h.members.GetRole(r.Context(), classID, userID)
	if role == "" {
		role = "owner"
	}
	detail.MyRole = role

	WriteJSON(w, http.StatusOK, detail)
}

func (h *Handler) updateClass(w http.ResponseWriter, r *http.Request, classID int64) {
	userID := userIDFromHeader(r)
	var in model.UpdateClassInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	class, err := h.classes.Update(r.Context(), principalFromRequest(r), classID, in)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	_ = h.activity.RecordEvent(r.Context(), model.ActivityEvent{
		UserID:     userID,
		EventType:  "class.updated",
		EntityType: "class",
		EntityID:   &class.ID,
		ClassID:    &class.ID,
		Metadata:   json.RawMessage(`{"className":"` + strings.ReplaceAll(class.Name, `"`, `\"`) + `"}`),
	})

	WriteJSON(w, http.StatusOK, class)
}

func (h *Handler) deleteClass(w http.ResponseWriter, r *http.Request, classID int64) {
	userID := userIDFromHeader(r)

	_ = h.activity.RecordEvent(r.Context(), model.ActivityEvent{
		UserID:     userID,
		EventType:  "class.deleted",
		EntityType: "class",
		EntityID:   &classID,
		ClassID:    &classID,
	})

	if err := h.classes.Delete(r.Context(), principalFromRequest(r), classID); err != nil {
		writeServiceError(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) joinClass(w http.ResponseWriter, r *http.Request, code string) {
	if r.Method != http.MethodPost {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID := userIDFromHeader(r)
	resp, err := h.members.JoinByCode(r.Context(), code, userID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	_ = h.activity.RecordEvent(r.Context(), model.ActivityEvent{
		UserID:     userID,
		EventType:  "class.member.joined",
		EntityType: "class",
		EntityID:   &resp.ClassID,
		ClassID:    &resp.ClassID,
		Metadata:   json.RawMessage(`{"className":"` + strings.ReplaceAll(resp.ClassName, `"`, `\"`) + `"}`),
	})

	WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) resetInviteCode(w http.ResponseWriter, r *http.Request, classID int64) {
	newCode, err := h.classes.ResetInviteCode(r.Context(), principalFromRequest(r), classID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"inviteCode": newCode})
}

// --- Dashboard handlers (Phase 3) ---

func (h *Handler) getClassDashboard(w http.ResponseWriter, r *http.Request, classID int64) {
	if h.dashboard == nil {
		WriteError(w, http.StatusServiceUnavailable, "dashboard unavailable")
		return
	}
	view, err := h.dashboard.Get(r.Context(), principalFromRequest(r), classID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, view)
}

func (h *Handler) setClassLeaderboard(w http.ResponseWriter, r *http.Request, classID int64) {
	if h.dashboard == nil {
		WriteError(w, http.StatusServiceUnavailable, "dashboard unavailable")
		return
	}
	var in struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := h.dashboard.SetLeaderboard(r.Context(), principalFromRequest(r), classID, in.Enabled); err != nil {
		writeServiceError(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]bool{"showLeaderboard": in.Enabled})
}

// --- Member handlers ---

func (h *Handler) listMembers(w http.ResponseWriter, r *http.Request, classID int64) {
	members, err := h.members.ListMembers(r.Context(), principalFromRequest(r), classID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	if members == nil {
		members = []*model.ClassMember{}
	}
	WriteJSON(w, http.StatusOK, members)
}

func (h *Handler) addMember(w http.ResponseWriter, r *http.Request, classID int64) {
	userID := userIDFromHeader(r)
	var in model.AddMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	member, err := h.members.AddMember(r.Context(), principalFromRequest(r), classID, in.UserID, in.Role)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	_ = h.activity.RecordEvent(r.Context(), model.ActivityEvent{
		UserID:     userID,
		EventType:  "class.member.added",
		EntityType: "member",
		EntityID:   &member.UserID,
		ClassID:    &classID,
		Metadata:   json.RawMessage(`{"role":"` + member.Role + `"}`),
	})

	WriteJSON(w, http.StatusCreated, member)
}

func (h *Handler) updateMemberRole(w http.ResponseWriter, r *http.Request, classID, targetUserID int64) {
	var in model.UpdateMemberRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if err := h.members.UpdateRole(r.Context(), principalFromRequest(r), classID, targetUserID, in.Role); err != nil {
		writeServiceError(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) removeMember(w http.ResponseWriter, r *http.Request, classID, targetUserID int64) {
	userID := userIDFromHeader(r)
	if err := h.members.RemoveMember(r.Context(), principalFromRequest(r), classID, targetUserID); err != nil {
		writeServiceError(w, r, err)
		return
	}

	_ = h.activity.RecordEvent(r.Context(), model.ActivityEvent{
		UserID:     userID,
		EventType:  "class.member.removed",
		EntityType: "member",
		EntityID:   &targetUserID,
		ClassID:    &classID,
	})

	WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) leaveClass(w http.ResponseWriter, r *http.Request, classID int64) {
	userID := userIDFromHeader(r)
	if err := h.members.LeaveClass(r.Context(), classID, userID); err != nil {
		writeServiceError(w, r, err)
		return
	}

	_ = h.activity.RecordEvent(r.Context(), model.ActivityEvent{
		UserID:     userID,
		EventType:  "class.member.left",
		EntityType: "member",
		ClassID:    &classID,
	})

	WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// --- Study set handlers ---

func (h *Handler) listClassStudySets(w http.ResponseWriter, r *http.Request, classID int64) {
	sets, err := h.studySets.ListStudySets(r.Context(), principalFromRequest(r), classID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	if sets == nil {
		sets = []*model.ClassStudySet{}
	}
	WriteJSON(w, http.StatusOK, sets)
}

func (h *Handler) addStudySetToClass(w http.ResponseWriter, r *http.Request, classID int64) {
	userID := userIDFromHeader(r)
	var in model.AddStudySetRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if err := h.studySets.AddStudySet(r.Context(), principalFromRequest(r), classID, in.StudySetID); err != nil {
		writeServiceError(w, r, err)
		return
	}
	if in.DueAt != nil {
		if _, err := h.db.ExecContext(r.Context(), `UPDATE class_study_sets SET due_at=$3 WHERE class_id=$1 AND study_set_id=$2`, classID, in.StudySetID, in.DueAt); err != nil {
			WriteError(w, 503, "deadline update unavailable")
			return
		}
	}

	_ = h.activity.RecordEvent(r.Context(), model.ActivityEvent{
		UserID:     userID,
		EventType:  "class.studyset.added",
		EntityType: "study_set",
		EntityID:   &in.StudySetID,
		ClassID:    &classID,
	})

	WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) removeStudySetFromClass(w http.ResponseWriter, r *http.Request, classID, studySetID int64) {
	userID := userIDFromHeader(r)
	if err := h.studySets.RemoveStudySet(r.Context(), principalFromRequest(r), classID, studySetID); err != nil {
		writeServiceError(w, r, err)
		return
	}

	_ = h.activity.RecordEvent(r.Context(), model.ActivityEvent{
		UserID:     userID,
		EventType:  "class.studyset.removed",
		EntityType: "study_set",
		EntityID:   &studySetID,
		ClassID:    &classID,
	})

	WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// --- Activity handlers ---

func (h *Handler) getActivityFeed(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromHeader(r)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 20
	}
	cursor := r.URL.Query().Get("cursor")

	feed, err := h.activity.GetFeed(r.Context(), userID, limit, cursor)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	WriteJSON(w, http.StatusOK, feed)
}

// --- Helpers ---

func userIDFromHeader(r *http.Request) int64 {
	raw := r.Header.Get("X-User-ID")
	if raw == "" {
		return 0
	}
	id, _ := strconv.ParseInt(raw, 10, 64)
	return id
}

func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, classservice.ErrUnauthorized):
		WriteErrorWithRequestID(w, r, http.StatusUnauthorized, "authentication required")
	case errors.Is(err, classservice.ErrForbidden):
		WriteErrorWithRequestID(w, r, http.StatusForbidden, "you do not have permission to perform this action")
	case errors.Is(err, classservice.ErrConflict):
		WriteErrorWithRequestID(w, r, http.StatusConflict, "resource already exists")
	case errors.Is(err, classservice.ErrNotFound):
		WriteErrorWithRequestID(w, r, http.StatusNotFound, "resource not found")
	case errors.Is(err, classservice.ErrValidation):
		WriteErrorWithRequestID(w, r, http.StatusUnprocessableEntity, "invalid request")
	default:
		msg := err.Error()
		if strings.Contains(msg, "not found") {
			WriteErrorWithRequestID(w, r, http.StatusNotFound, msg)
		} else if strings.Contains(msg, "full") {
			WriteErrorWithRequestID(w, r, http.StatusConflict, msg)
		} else if strings.Contains(msg, "already") {
			WriteErrorWithRequestID(w, r, http.StatusConflict, msg)
		} else {
			WriteErrorWithRequestID(w, r, http.StatusInternalServerError, "internal server error")
		}
	}
}
