package service

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/packages/authz"
	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/repository"
)

// ErrUnauthorized is returned when no verified actor is present.
var ErrUnauthorized = errors.New("unauthorized")

// --- response views (JSON) ---

type RoleView struct {
	Policy      *authz.Policy `json:"policy,omitempty"`
	ID          int64         `json:"id"`
	Key         string        `json:"key"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Kind        string        `json:"kind"`
	OrgID       *int64        `json:"orgId,omitempty"`
}

type AssignmentView struct {
	ID            int64      `json:"id"`
	PrincipalType string     `json:"principalType"`
	PrincipalID   int64      `json:"principalId"`
	RoleKey       string     `json:"roleKey"`
	ScopeType     string     `json:"scopeType"`
	ScopeID       *int64     `json:"scopeId,omitempty"`
	CreatedBy     int64      `json:"createdBy"`
	CreatedAt     time.Time  `json:"createdAt"`
	ExpiresAt     *time.Time `json:"expiresAt,omitempty"`
}

type AdminUserView struct {
	PlatformRoles []string  `json:"platformRoles"`
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Email         string    `json:"email"`
	Role          string    `json:"role"`
	Disabled      bool      `json:"disabled"`
	CreatedAt     time.Time `json:"createdAt"`
}

type AuditEntryView struct {
	ID        int64           `json:"id"`
	ActorID   int64           `json:"actorId"`
	Action    string          `json:"action"`
	Resource  string          `json:"resource"`
	Result    string          `json:"result"`
	RequestID string          `json:"requestId,omitempty"`
	IP        string          `json:"ip,omitempty"`
	Detail    json.RawMessage `json:"detail,omitempty"`
	At        time.Time       `json:"at"`
}

// --- authorization helper ---

// requirePlatform denies by default unless the actor holds the action at platform scope.
func requirePlatform(actor authz.Principal, action authz.Action) error {
	if actor.UserID <= 0 {
		return ErrUnauthorized
	}
	if !authz.Can(actor, action, authz.Resource{Type: "platform"}) {
		return ErrForbidden
	}
	return nil
}

// auditDenied records a rejected privileged attempt so operators can spot escalation probes.
func (s *AuthService) auditDenied(ctx context.Context, actor authz.Principal, action, resource, requestID, ip string) {
	if actor.UserID <= 0 {
		return
	}
	_ = s.repo.InsertAuditLog(ctx, actor.UserID, action, resource, "deny", requestID, ip, nil)
}

// --- roles ---

func (s *AuthService) ListRoles(ctx context.Context, actor authz.Principal, requestID, ip string) ([]RoleView, error) {
	if err := requirePlatform(actor, authz.ActionIAMListRoles); err != nil {
		s.auditDenied(ctx, actor, string(authz.ActionIAMListRoles), "hq:platform", requestID, ip)
		return nil, err
	}
	roles, err := s.repo.ListRoles(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]RoleView, 0, len(roles))
	for _, r := range roles {
		view := RoleView{ID: r.ID, Key: r.Key, Name: r.Name, Description: r.Description, Kind: r.Kind, OrgID: r.OrgID}
		if policy, ok := authz.ManagedPolicy(r.Key); ok {
			view.Policy = &policy
		}
		out = append(out, view)
	}
	return out, nil
}

// --- assignments ---

func (s *AuthService) ListAssignments(ctx context.Context, actor authz.Principal, principalID int64, roleKey string, limit, offset int, requestID, ip string) ([]AssignmentView, error) {
	if err := requirePlatform(actor, authz.ActionIAMListAssignments); err != nil {
		s.auditDenied(ctx, actor, string(authz.ActionIAMListAssignments), "hq:platform", requestID, ip)
		return nil, err
	}
	rows, err := s.repo.ListAssignments(ctx, principalID, roleKey, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]AssignmentView, 0, len(rows))
	for _, a := range rows {
		out = append(out, assignmentView(a))
	}
	return out, nil
}

// AuthorizeAssignRole lets the HTTP layer fail closed before parsing a request body.
// It performs the same check and deny-audit as AssignRole, which then re-checks after decoding
// (a caller that passes here is never denied there, so deny entries are not duplicated).
func (s *AuthService) AuthorizeAssignRole(ctx context.Context, actor authz.Principal, requestID, ip string) error {
	if err := requirePlatform(actor, authz.ActionIAMAssignRole); err != nil {
		s.auditDenied(ctx, actor, string(authz.ActionIAMAssignRole), "hq:platform", requestID, ip)
		return err
	}
	return nil
}

// AssignRole grants a managed role to a user. Only actors holding iam:AssignRole may call it,
// and every attempt (allow or deny) is written to the audit log.
func (s *AuthService) AssignRole(ctx context.Context, actor authz.Principal, targetID int64, roleKey, scopeType string, scopeID *int64, requestID, ip string) (AssignmentView, error) {
	resource := auditResourceUser(targetID)
	if err := requirePlatform(actor, authz.ActionIAMAssignRole); err != nil {
		s.auditDenied(ctx, actor, string(authz.ActionIAMAssignRole), resource, requestID, ip)
		return AssignmentView{}, err
	}
	if targetID <= 0 {
		return AssignmentView{}, &ValidationError{Field: "userId", Msg: "userId is required"}
	}
	if strings.HasPrefix(roleKey, "Class") {
		return AssignmentView{}, &ValidationError{Field: "roleKey", Msg: "class roles are managed through class membership"}
	}
	if !isManagedRoleKey(roleKey) {
		return AssignmentView{}, &ValidationError{Field: "roleKey", Msg: "unknown role"}
	}
	scopeType = normalizeScope(scopeType)
	if scopeType == "" {
		return AssignmentView{}, &ValidationError{Field: "scopeType", Msg: "scopeType must be platform or org"}
	}
	if scopeType == "org" && (scopeID == nil || *scopeID <= 0) {
		return AssignmentView{}, &ValidationError{Field: "scopeId", Msg: "scopeId is required for org scope"}
	}
	if scopeType == "platform" {
		scopeID = nil
	}
	if err := s.validateOrgScopeAssignment(ctx, roleKey, scopeType, scopeID); err != nil {
		return AssignmentView{}, err
	}

	if store, ok := s.repo.(interface {
		CreateAssignmentAudited(context.Context, repository.IAMAssignment, string, string) (repository.IAMAssignment, error)
	}); ok {
		created, err := store.CreateAssignmentAudited(ctx, repository.IAMAssignment{PrincipalType: "user", PrincipalID: targetID, RoleKey: roleKey, ScopeType: scopeType, ScopeID: scopeID, CreatedBy: actor.UserID}, requestID, ip)
		return assignmentView(created), err
	}
	created, err := s.repo.CreateAssignment(ctx, repository.IAMAssignment{
		PrincipalType: "user",
		PrincipalID:   targetID,
		RoleKey:       roleKey,
		ScopeType:     scopeType,
		ScopeID:       scopeID,
		CreatedBy:     actor.UserID,
	})
	if err != nil {
		return AssignmentView{}, err
	}
	_ = s.repo.InsertAuditLog(ctx, actor.UserID, string(authz.ActionIAMAssignRole), resource, "allow", requestID, ip, map[string]any{
		"roleKey": roleKey, "scopeType": scopeType, "assignmentId": created.ID,
	})
	return assignmentView(created), nil
}

// RevokeRole removes a role assignment by id.
func (s *AuthService) RevokeRole(ctx context.Context, actor authz.Principal, assignmentID int64, requestID, ip string) error {
	if err := requirePlatform(actor, authz.ActionIAMRevokeRole); err != nil {
		s.auditDenied(ctx, actor, string(authz.ActionIAMRevokeRole), "hq:assignment/"+strconv.FormatInt(assignmentID, 10), requestID, ip)
		return err
	}
	if store, ok := s.repo.(interface {
		DeleteAssignmentAudited(context.Context, int64, int64, string, string) error
	}); ok {
		return store.DeleteAssignmentAudited(ctx, assignmentID, actor.UserID, requestID, ip)
	}
	existing, err := s.repo.GetAssignment(ctx, assignmentID)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteAssignment(ctx, assignmentID); err != nil {
		return err
	}
	_ = s.repo.InsertAuditLog(ctx, actor.UserID, string(authz.ActionIAMRevokeRole), auditResourceUser(existing.PrincipalID), "allow", requestID, ip, map[string]any{
		"roleKey": existing.RoleKey, "scopeType": existing.ScopeType, "assignmentId": assignmentID,
	})
	return nil
}

// --- users ---

func (s *AuthService) ListAdminUsers(ctx context.Context, actor authz.Principal, search, role string, limit, offset int, requestID, ip string, status ...string) ([]AdminUserView, int, error) {
	if err := requirePlatform(actor, authz.ActionUserList); err != nil {
		s.auditDenied(ctx, actor, string(authz.ActionUserList), "hq:user/*", requestID, ip)
		return nil, 0, err
	}
	filter := ""
	if len(status) > 0 {
		filter = status[0]
	}
	if filter != "" && filter != "active" && filter != "disabled" {
		return nil, 0, &ValidationError{Field: "status", Msg: "invalid status"}
	}
	users, total, err := s.repo.ListUsers(ctx, search, role, filter, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	out := make([]AdminUserView, 0, len(users))
	for _, u := range users {
		roles, err := s.repo.ListPlatformRoleKeys(ctx, u.ID)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, AdminUserView{PlatformRoles: roles, ID: u.ID, Name: u.Name, Email: u.Email, Role: u.Role, Disabled: u.Disabled, CreatedAt: u.CreatedAt})
	}
	return out, total, nil
}

// SetUserDisabled enables or disables an account. Administrators cannot disable themselves.
func (s *AuthService) SetUserDisabled(ctx context.Context, actor authz.Principal, targetID int64, disabled bool, requestID, ip string) (AdminUserView, error) {
	resource := auditResourceUser(targetID)
	action := authz.ActionUserDisable
	if err := requirePlatform(actor, action); err != nil {
		s.auditDenied(ctx, actor, string(action), resource, requestID, ip)
		return AdminUserView{}, err
	}
	if targetID == actor.UserID {
		return AdminUserView{}, &ValidationError{Field: "userId", Msg: "cannot change your own account status"}
	}
	if store, ok := s.repo.(interface {
		SetUserDisabledAudited(context.Context, int64, int64, bool, string, string) (repository.AdminUser, error)
	}); ok {
		u, err := store.SetUserDisabledAudited(ctx, targetID, actor.UserID, disabled, requestID, ip)
		return AdminUserView{ID: u.ID, Name: u.Name, Email: u.Email, Role: u.Role, Disabled: u.Disabled, CreatedAt: u.CreatedAt}, err
	}
	u, err := s.repo.SetUserDisabled(ctx, targetID, disabled)
	if err != nil {
		return AdminUserView{}, err
	}
	_ = s.repo.InsertAuditLog(ctx, actor.UserID, string(action), resource, "allow", requestID, ip, map[string]any{
		"disabled": disabled,
	})
	return AdminUserView{ID: u.ID, Name: u.Name, Email: u.Email, Role: u.Role, Disabled: u.Disabled, CreatedAt: u.CreatedAt}, nil
}

// --- audit log ---

func (s *AuthService) ListAudit(ctx context.Context, actor authz.Principal, actorID int64, action string, limit, offset int, requestID, ip string) ([]AuditEntryView, error) {
	if err := requirePlatform(actor, authz.ActionAuditRead); err != nil {
		s.auditDenied(ctx, actor, string(authz.ActionAuditRead), "hq:audit/*", requestID, ip)
		return nil, err
	}
	entries, err := s.repo.ListAuditLog(ctx, actorID, action, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]AuditEntryView, 0, len(entries))
	for _, e := range entries {
		view := AuditEntryView{
			ID: e.ID, ActorID: e.ActorID, Action: e.Action, Resource: e.Resource,
			Result: e.Result, At: e.At, Detail: json.RawMessage(e.Detail),
		}
		if e.RequestID != nil {
			view.RequestID = *e.RequestID
		}
		if e.IP != nil {
			view.IP = *e.IP
		}
		out = append(out, view)
	}
	return out, nil
}

// --- internal directory ---

// UserDirectoryEntry is the minimal user record exposed to trusted internal callers
// (class dashboard resolving member names). Never exposed to browsers.
type UserDirectoryEntry struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

const maxDirectoryIDs = 200

// UserDirectory resolves user ids to display records for service-to-service calls.
func (s *AuthService) UserDirectory(ctx context.Context, ids []int64) ([]UserDirectoryEntry, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	if len(ids) > maxDirectoryIDs {
		return nil, &ValidationError{Field: "ids", Msg: "too many ids"}
	}
	users, err := s.repo.ListUsersByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]UserDirectoryEntry, 0, len(users))
	for _, u := range users {
		out = append(out, UserDirectoryEntry{ID: u.ID, Name: u.Name, Email: u.Email, Role: u.Role})
	}
	return out, nil
}

// --- helpers ---

func assignmentView(a repository.IAMAssignment) AssignmentView {
	return AssignmentView{
		ID: a.ID, PrincipalType: a.PrincipalType, PrincipalID: a.PrincipalID, RoleKey: a.RoleKey,
		ScopeType: a.ScopeType, ScopeID: a.ScopeID, CreatedBy: a.CreatedBy, CreatedAt: a.CreatedAt, ExpiresAt: a.ExpiresAt,
	}
}

func isManagedRoleKey(key string) bool {
	for _, managed := range authz.ManagedRoleKeys() {
		if managed == key {
			return true
		}
	}
	return false
}

func normalizeScope(scope string) string {
	switch scope {
	case "":
		return "platform"
	case "platform", "org":
		return scope
	default:
		return ""
	}
}
