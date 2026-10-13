package http

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/packages/authz"
	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/repository"
	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/service"
)

// NewRouter wires up all auth routes.
func NewRouter(svc *service.AuthService, db *sql.DB) http.Handler {
	mux := http.NewServeMux()

	// health
	mux.HandleFunc("GET /healthz", healthHandler("auth", db))

	// auth lifecycle – P2-AUTH-01
	mux.HandleFunc("GET /v1/auth/me", meHandler(svc))
	mux.HandleFunc("POST /v1/auth/register", registerHandler(svc))
	mux.HandleFunc("POST /v1/auth/login", loginHandler(svc))
	mux.HandleFunc("POST /v1/auth/reauth", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Password string `json:"password"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&in) != nil {
			writeError(w, 400, "invalid_json", "invalid body")
			return
		}
		u, err := svc.ConfirmPassword(r.Context(), bearerToken(r), in.Password)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		identity, err := svc.VerifyToken(r.Context(), bearerToken(r))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"userId": u.ID, "role": identity.Role, "platformRoles": identity.PlatformRoles})
	})
	mux.HandleFunc("POST /v1/auth/logout", logoutHandler(svc))
	mux.HandleFunc("POST /v1/auth/logout-all", logoutAllHandler(svc))
	mux.HandleFunc("POST /v1/auth/refresh", refreshHandler(svc))

	// internal token verify for gateway/study – P2-AUTH-02
	mux.HandleFunc("GET /internal/auth/verify", verifyHandler(svc))
	// internal user directory for trusted services (class dashboard) – Phase 3
	mux.HandleFunc("GET /internal/users", internalUsersHandler(svc))

	// IAM permissions for web UI
	mux.HandleFunc("GET /v1/me/permissions", permissionsHandler(svc))

	// Admin IAM console (Phase 1 / IAM-09). Gateway performs a coarse admin check; the
	// service re-checks every action with authz.Can and writes an audit entry.
	mux.HandleFunc("GET /v1/admin/overview", adminOverviewHandler(svc))
	mux.HandleFunc("GET /v1/admin/users/stats", authz.DailyMetricsHandler(db, "auth"))
	mux.HandleFunc("GET /v1/admin/roles", adminListRolesHandler(svc))
	mux.HandleFunc("GET /v1/admin/assignments", adminListAssignmentsHandler(svc))
	mux.HandleFunc("POST /v1/admin/assignments", adminAssignRoleHandler(svc))
	mux.HandleFunc("DELETE /v1/admin/assignments/{id}", adminRevokeRoleHandler(svc))
	mux.HandleFunc("GET /v1/admin/users", adminListUsersHandler(svc))
	mux.HandleFunc("GET /v1/admin/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id < 1 {
			writeError(w, 422, "invalid_id", "invalid user ID")
			return
		}
		u, err := svc.GetAdminUser(r.Context(), principalFromRequest(r), id)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, 200, u)
	})
	mux.HandleFunc("POST /v1/admin/users/{id}/disable", adminSetUserDisabledHandler(svc, true))
	mux.HandleFunc("POST /v1/admin/users/{id}/enable", adminSetUserDisabledHandler(svc, false))
	mux.HandleFunc("GET /v1/admin/audit", adminAuditHandler(svc))

	// user profile – P2-AUTH-03
	mux.HandleFunc("GET /v1/auth/profile", profileGetHandler(svc))
	mux.HandleFunc("PATCH /v1/auth/profile", profilePatchHandler(svc))
	mux.HandleFunc("PUT /v1/auth/profile", profilePatchHandler(svc))

	// Phase 10: TTS & Languages
	ttsSvc := service.NewTTSService()
	mux.HandleFunc("GET /v1/tts", ttsHandler(ttsSvc))
	mux.HandleFunc("GET /v1/languages", languagesHandler())

	return loggingMiddleware(requestIDMiddleware(mux))
}

// --- handlers ---

func permissionsHandler(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		classID, _ := strconv.ParseInt(r.URL.Query().Get("classId"), 10, 64)
		resp, err := svc.Permissions(r.Context(), bearerToken(r), classID)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// principalFromRequest builds the IAM principal from gateway-injected identity headers.
// The gateway strips these headers from client requests before injecting verified values.
func principalFromRequest(r *http.Request) authz.Principal {
	id, _ := strconv.ParseInt(r.Header.Get("X-User-ID"), 10, 64)
	return authz.Principal{
		UserID:        id,
		LegacyRole:    r.Header.Get("X-User-Role"),
		PlatformRoles: authz.ParsePlatformRoles(r.Header.Get("X-User-Roles")),
	}
}

func clientIP(r *http.Request) string {
	address := r.RemoteAddr
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		address = strings.SplitN(fwd, ",", 2)[0]
	}
	address = strings.TrimSpace(address)
	if host, _, err := net.SplitHostPort(address); err == nil {
		address = host
	}
	if ip := net.ParseIP(address); ip != nil {
		return ip.String()
	}
	// An invalid forwarding value must not prevent PostgreSQL from recording the audit.
	return ""
}

func listParams(r *http.Request) (limit, offset int) {
	limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ = strconv.Atoi(r.URL.Query().Get("offset"))
	return limit, offset
}

// parseRangeDays accepts "7d" / "28" and returns 0 for anything unparseable (service default).
func parseRangeDays(raw string) int {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return 0
	}
	raw = strings.TrimSuffix(raw, "d")
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return n
}

func adminOverviewHandler(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		view, err := svc.AdminOverview(r.Context(), principalFromRequest(r), parseRangeDays(r.URL.Query().Get("range")), requestID(w), clientIP(r))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, view)
	}
}

func adminListRolesHandler(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roles, err := svc.ListRoles(r.Context(), principalFromRequest(r), requestID(w), clientIP(r))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"roles": roles})
	}
}

func adminListAssignmentsHandler(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principalID, _ := strconv.ParseInt(r.URL.Query().Get("userId"), 10, 64)
		limit, offset := listParams(r)
		items, err := svc.ListAssignments(r.Context(), principalFromRequest(r), principalID, r.URL.Query().Get("roleKey"), limit, offset, requestID(w), clientIP(r))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"assignments": items})
	}
}

func adminAssignRoleHandler(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor := principalFromRequest(r)
		// Authorize before reading the body so an unauthorized caller never gets 400/422 feedback.
		if err := svc.AuthorizeAssignRole(r.Context(), actor, requestID(w), clientIP(r)); err != nil {
			writeServiceError(w, err)
			return
		}
		var in struct {
			UserID    int64  `json:"userId"`
			RoleKey   string `json:"roleKey"`
			ScopeType string `json:"scopeType"`
			ScopeID   *int64 `json:"scopeId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid JSON body")
			return
		}
		if in.RoleKey == authz.RolePlatformAdmin {
			u, err := svc.ConfirmPassword(r.Context(), bearerToken(r), r.Header.Get("X-Admin-Password"))
			if err != nil {
				writeServiceError(w, err)
				return
			}
			if u.ID != actor.UserID {
				writeError(w, 403, "forbidden", "identity mismatch")
				return
			}
		}
		created, err := svc.AssignRole(r.Context(), actor, in.UserID, in.RoleKey, in.ScopeType, in.ScopeID, requestID(w), clientIP(r))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, created)
	}
}

func adminRevokeRoleHandler(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id <= 0 {
			writeError(w, http.StatusBadRequest, "invalid_id", "invalid assignment id")
			return
		}
		if err := svc.RevokeRole(r.Context(), principalFromRequest(r), id, requestID(w), clientIP(r)); err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

func adminListUsersHandler(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, offset := listParams(r)
		users, total, err := svc.ListAdminUsers(r.Context(), principalFromRequest(r), r.URL.Query().Get("search"), r.URL.Query().Get("role"), limit, offset, requestID(w), clientIP(r), r.URL.Query().Get("status"))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"users": users, "total": total})
	}
}

func adminSetUserDisabledHandler(svc *service.AuthService, disabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id <= 0 {
			writeError(w, http.StatusBadRequest, "invalid_id", "invalid user id")
			return
		}
		u, err := svc.SetUserDisabled(r.Context(), principalFromRequest(r), id, disabled, requestID(w), clientIP(r))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, u)
	}
}

func adminAuditHandler(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, _ := strconv.ParseInt(r.URL.Query().Get("actorId"), 10, 64)
		limit, offset := listParams(r)
		entries, err := svc.ListAudit(r.Context(), principalFromRequest(r), actorID, r.URL.Query().Get("action"), limit, offset, requestID(w), clientIP(r))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
	}
}

// internalUsersHandler resolves user ids to display records for trusted services.
// Requires the shared internal token; never reachable through the public gateway.
func internalUsersHandler(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !internalTokenOK(r) {
			writeError(w, http.StatusUnauthorized, "invalid_internal_token", "invalid internal token")
			return
		}
		ids := parseIDList(r.URL.Query().Get("ids"))
		users, err := svc.UserDirectory(r.Context(), ids)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"users": users})
	}
}

func meHandler(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, err := svc.Me(r.Context(), bearerToken(r))
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]any{"authenticated": false, "user": nil})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "user": u})
	}
}

func registerHandler(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input model.RegisterInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid JSON body")
			return
		}
		resp, err := svc.Register(r.Context(), input)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, resp)
	}
}

func loginHandler(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input model.LoginInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid JSON body")
			return
		}
		resp, err := svc.Login(r.Context(), input)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func logoutHandler(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		svc.Logout(r.Context(), bearerToken(r))
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// logoutAllHandler – P2-AUTH-01: invalidates all sessions for this user.
func logoutAllHandler(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := svc.LogoutAll(r.Context(), bearerToken(r)); err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

func refreshHandler(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := svc.Refresh(r.Context(), bearerToken(r))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// verifyHandler – P2-AUTH-02: internal endpoint for gateway/study to verify tokens.
// Returns user identity so study service can read userId from X-Auth-User-Id header
// set by gateway after calling this endpoint, OR reads directly via this endpoint.
func verifyHandler(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, err := svc.VerifyToken(r.Context(), bearerToken(r))
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
			return
		}
		writeJSON(w, http.StatusOK, struct {
			Authenticated bool `json:"authenticated"`
			model.VerifiedIdentity
		}{Authenticated: true, VerifiedIdentity: identity})
	}
}

// profileGetHandler – P2-AUTH-03: get current user's profile.
func profileGetHandler(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, err := svc.GetProfile(r.Context(), bearerToken(r))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, u)
	}
}

// profilePatchHandler – P2-AUTH-03: update name/image.
func profilePatchHandler(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input model.UpdateProfileInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid JSON body")
			return
		}
		u, err := svc.UpdateProfile(r.Context(), bearerToken(r), input)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, u)
	}
}

func healthHandler(serviceName string, db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body := map[string]string{"service": serviceName, "status": "ok", "database": "ok"}
		status := http.StatusOK
		if err := db.PingContext(r.Context()); err != nil {
			status = http.StatusServiceUnavailable
			body["status"] = "degraded"
			body["database"] = "offline"
		}
		writeJSON(w, status, body)
	}
}

// --- P2-AUTH-04: standardized error responses ---
//
// Canonical error envelope shared across ALL Auth endpoints.
// Dev2 (Study) and Dev5 (Integration) must adopt the same shape.
//
// Normal error:
//   { "code": "<snake_case_code>", "message": "<human readable>" }
//
// Validation error (422 only) stores field-level context under details.field.

// apiError is the canonical error envelope for all Auth error responses.
type apiError struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"requestId"`
	Details   map[string]any `json:"details"`
}

// apiValidationError uses the same envelope and places validation metadata in details.
type apiValidationError struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"requestId"`
	Details   map[string]any `json:"details"`
}

// writeError sends a structured JSON error using the canonical envelope.
func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, apiError{Code: strings.ToUpper(code), Message: msg, RequestID: requestID(w), Details: map[string]any{}})
}

// writeServiceError maps service-layer errors to correct HTTP status + canonical code.
// P2-AUTH-04: every branch must emit {code, message} — never a bare string or {error}.
func writeServiceError(w http.ResponseWriter, err error) {
	var ve *service.ValidationError
	switch {
	case errors.As(err, &ve):
		writeJSON(w, http.StatusUnprocessableEntity, apiValidationError{
			Code: "VALIDATION_ERROR", Message: ve.Msg, RequestID: requestID(w),
			Details: map[string]any{"field": ve.Field},
		})
	case errors.Is(err, service.ErrEmailTaken):
		writeError(w, http.StatusConflict, "email_taken", "email already registered")
	case errors.Is(err, service.ErrInvalidCredential):
		writeError(w, http.StatusUnauthorized, "invalid_credential", "invalid email or password")
	case errors.Is(err, service.ErrInvalidSession),
		errors.Is(err, service.ErrExpiredSession),
		errors.Is(err, service.ErrRevokedSession),
		errors.Is(err, service.ErrDisabledUser):
		writeError(w, http.StatusUnauthorized, "invalid_session", "invalid or expired session")
	case errors.Is(err, repository.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
	case errors.Is(err, service.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
	case errors.Is(err, service.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "access denied")
	default:
		log.Printf("[auth] unexpected error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

// --- Phase 10: TTS & Languages handlers ---

func ttsHandler(ttsSvc *service.TTSService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		text := r.URL.Query().Get("text")
		lang := r.URL.Query().Get("lang")
		if text == "" {
			writeError(w, http.StatusUnprocessableEntity, "validation_error", "text is required")
			return
		}
		if lang == "" {
			lang = "en-US"
		}
		audio, err := ttsSvc.GetAudio(text, lang)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "TTS_UNAVAILABLE", err.Error())
			return
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(audio)
	}
}

func languagesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, service.Languages)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("[auth] writeJSON encode error: %v", err)
	}
}

// --- middleware ---

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = time.Now().UTC().Format("20060102150405.000000000")
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r)
	})
}

func requestID(w http.ResponseWriter) string {
	return w.Header().Get("X-Request-ID")
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("[auth] %s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}
