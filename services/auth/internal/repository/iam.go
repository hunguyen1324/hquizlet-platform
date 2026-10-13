package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/packages/authz"
)

type IAMAssignment struct {
	ID            int64
	PrincipalType string
	PrincipalID   int64
	RoleKey       string
	ScopeType     string
	ScopeID       *int64
	CreatedBy     int64
	CreatedAt     time.Time
	ExpiresAt     *time.Time
}

func (r *AuthRepository) ListPlatformRoleKeys(ctx context.Context, userID int64) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT role_key FROM iam_assignments
		 WHERE principal_type = 'user' AND principal_id = $1 AND scope_type = 'platform'
		   AND (expires_at IS NULL OR expires_at > now())`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// ListOrgRoleEntries returns the caller's organization memberships as "<orgID>:<RoleKey>"
// wire entries, ready to be signed into X-User-Org-Roles for downstream services.
func (r *AuthRepository) ListOrgRoleEntries(ctx context.Context, userID int64) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT scope_id, role_key FROM iam_assignments
		 WHERE principal_type = 'user' AND principal_id = $1 AND scope_type = 'org' AND scope_id IS NOT NULL
		   AND (expires_at IS NULL OR expires_at > now())
		 ORDER BY scope_id, role_key`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var orgID int64
		var roleKey string
		if err := rows.Scan(&orgID, &roleKey); err != nil {
			return nil, err
		}
		out = append(out, authz.FormatOrgRole(orgID, roleKey))
	}
	return out, rows.Err()
}

// Organization is a school/company that owns classes.
type Organization struct {
	ID          int64
	Name        string
	Slug        string
	Kind        string
	OwnerUserID int64
	CreatedAt   time.Time
}

// OrganizationExists reports whether an organization id is present.
func (r *AuthRepository) OrganizationExists(ctx context.Context, id int64) (bool, error) {
	var one int
	err := r.db.QueryRowContext(ctx, `SELECT 1 FROM organizations WHERE id = $1`, id).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// ListOrganizations returns organizations newest first.
func (r *AuthRepository) ListOrganizations(ctx context.Context, limit, offset int) ([]Organization, int, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM organizations`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, slug, kind, owner_user_id, created_at FROM organizations
		 ORDER BY id DESC LIMIT $1 OFFSET $2`, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Organization
	for rows.Next() {
		var o Organization
		if err := rows.Scan(&o.ID, &o.Name, &o.Slug, &o.Kind, &o.OwnerUserID, &o.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, o)
	}
	return out, total, rows.Err()
}

// CreateOrganization inserts an organization and returns the stored row.
// A duplicate slug is reported as ErrConflict instead of a driver-specific error.
func (r *AuthRepository) CreateOrganization(ctx context.Context, name, slug, kind string, ownerUserID int64) (Organization, error) {
	var o Organization
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO organizations (name, slug, kind, owner_user_id)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (slug) DO NOTHING
		 RETURNING id, name, slug, kind, owner_user_id, created_at`,
		name, slug, kind, ownerUserID,
	).Scan(&o.ID, &o.Name, &o.Slug, &o.Kind, &o.OwnerUserID, &o.CreatedAt)
	if err == sql.ErrNoRows {
		return Organization{}, ErrConflict
	}
	if err != nil {
		return Organization{}, err
	}
	return o, nil
}

func (r *AuthRepository) CreatePlatformAssignment(ctx context.Context, userID int64, roleKey string, createdBy int64) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO iam_assignments (principal_type, principal_id, role_key, scope_type, scope_id, created_by)
		 VALUES ('user', $1, $2, 'platform', NULL, $3)
		 ON CONFLICT (principal_type, principal_id, role_key, scope_type, scope_id) DO NOTHING`,
		userID, roleKey, createdBy,
	)
	return err
}

func (r *AuthRepository) InsertAuditLog(ctx context.Context, actorID int64, action, resource, result, requestID, ip string, detail map[string]any) error {
	var detailJSON []byte
	if detail != nil {
		var err error
		detailJSON, err = json.Marshal(detail)
		if err != nil {
			return err
		}
	}
	ip = strings.TrimSpace(ip)
	var ipArg any
	if ip != "" {
		ipArg = ip
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO audit_log (actor_id, action, resource, result, request_id, ip, detail)
		 VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6::inet, $7::jsonb)`,
		actorID, action, resource, result, requestID, ipArg, detailJSON,
	)
	return err
}

func (r *AuthRepository) SeedManagedRoles(ctx context.Context) error {
	for _, key := range authz.ManagedRoleKeys() {
		name := key
		_, err := r.db.ExecContext(ctx,
			`INSERT INTO iam_roles (key, name, description, kind, org_id)
			 VALUES ($1, $2, $3, 'managed', NULL)
			 ON CONFLICT (key, org_id) DO NOTHING`,
			key, name, "Managed platform role",
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// --- Admin console reads (dashboard plan Phase 1 / IAM-09) ---

// IAMRole is a managed or custom role definition.
type IAMRole struct {
	ID          int64
	Key         string
	Name        string
	Description string
	Kind        string
	OrgID       *int64
	CreatedAt   time.Time
}

// AdminUser is a user record for the admin user list (includes disabled state).
type AdminUser struct {
	ID        int64
	Name      string
	Email     string
	Role      string
	Disabled  bool
	CreatedAt time.Time
}

// AuditEntry is a row of the audit log.
type AuditEntry struct {
	ID        int64
	ActorID   int64
	Action    string
	Resource  string
	Result    string
	RequestID *string
	IP        *string
	Detail    []byte
	At        time.Time
}

// ListRoles returns every role definition (managed + custom).
func (r *AuthRepository) ListRoles(ctx context.Context) ([]IAMRole, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, key, name, description, kind, org_id, created_at
		 FROM iam_roles ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []IAMRole
	for rows.Next() {
		var role IAMRole
		if err := rows.Scan(&role.ID, &role.Key, &role.Name, &role.Description, &role.Kind, &role.OrgID, &role.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, role)
	}
	return out, rows.Err()
}

// ListAssignments returns role assignments, optionally filtered by principal or role key.
func (r *AuthRepository) ListAssignments(ctx context.Context, principalID int64, roleKey string, limit, offset int) ([]IAMAssignment, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, principal_type, principal_id, role_key, scope_type, scope_id, created_by, created_at, expires_at
		 FROM iam_assignments
		 WHERE ($1 = 0 OR principal_id = $1) AND ($2 = '' OR role_key = $2)
		 ORDER BY id DESC LIMIT $3 OFFSET $4`,
		principalID, roleKey, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []IAMAssignment
	for rows.Next() {
		var a IAMAssignment
		if err := rows.Scan(&a.ID, &a.PrincipalType, &a.PrincipalID, &a.RoleKey, &a.ScopeType, &a.ScopeID, &a.CreatedBy, &a.CreatedAt, &a.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetAssignment returns a single assignment by id.
func (r *AuthRepository) GetAssignment(ctx context.Context, id int64) (IAMAssignment, error) {
	var a IAMAssignment
	err := r.db.QueryRowContext(ctx,
		`SELECT id, principal_type, principal_id, role_key, scope_type, scope_id, created_by, created_at, expires_at
		 FROM iam_assignments WHERE id = $1`, id,
	).Scan(&a.ID, &a.PrincipalType, &a.PrincipalID, &a.RoleKey, &a.ScopeType, &a.ScopeID, &a.CreatedBy, &a.CreatedAt, &a.ExpiresAt)
	if err != nil {
		return IAMAssignment{}, ErrNotFound
	}
	return a, nil
}

// CreateAssignment inserts a role assignment (idempotent per unique key) and returns the stored row.
func (r *AuthRepository) CreateAssignment(ctx context.Context, a IAMAssignment) (IAMAssignment, error) {
	var out IAMAssignment
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO iam_assignments (principal_type, principal_id, role_key, scope_type, scope_id, created_by, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (principal_type, principal_id, role_key, scope_type, scope_id) DO NOTHING
		 RETURNING id, principal_type, principal_id, role_key, scope_type, scope_id, created_by, created_at, expires_at`,
		a.PrincipalType, a.PrincipalID, a.RoleKey, a.ScopeType, a.ScopeID, a.CreatedBy, a.ExpiresAt,
	).Scan(&out.ID, &out.PrincipalType, &out.PrincipalID, &out.RoleKey, &out.ScopeType, &out.ScopeID, &out.CreatedBy, &out.CreatedAt, &out.ExpiresAt)
	if err == sql.ErrNoRows {
		return r.findAssignment(ctx, a)
	}
	if err != nil {
		return IAMAssignment{}, err
	}
	return out, nil
}

func (r *AuthRepository) findAssignment(ctx context.Context, a IAMAssignment) (IAMAssignment, error) {
	var out IAMAssignment
	err := r.db.QueryRowContext(ctx,
		`SELECT id, principal_type, principal_id, role_key, scope_type, scope_id, created_by, created_at, expires_at
		 FROM iam_assignments
		 WHERE principal_type = $1 AND principal_id = $2 AND role_key = $3 AND scope_type = $4
		   AND scope_id IS NOT DISTINCT FROM $5`,
		a.PrincipalType, a.PrincipalID, a.RoleKey, a.ScopeType, a.ScopeID,
	).Scan(&out.ID, &out.PrincipalType, &out.PrincipalID, &out.RoleKey, &out.ScopeType, &out.ScopeID, &out.CreatedBy, &out.CreatedAt, &out.ExpiresAt)
	if err != nil {
		return IAMAssignment{}, ErrNotFound
	}
	return out, nil
}

// DeleteAssignment removes a role assignment. Returns ErrNotFound when nothing matched.
func (r *AuthRepository) DeleteAssignment(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM iam_assignments WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListUsers returns users filtered by search term and legacy role, plus the total count.
func (r *AuthRepository) ListUsers(ctx context.Context, search, role, status string, limit, offset int) ([]AdminUser, int, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var total int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users
		 WHERE ($1 = '' OR name ILIKE '%' || $1 || '%' OR email ILIKE '%' || $1 || '%')
		   AND ($3 = '' OR disabled=($3='disabled'))
 AND ($2 = '' OR role = $2 OR EXISTS (SELECT 1 FROM iam_assignments a WHERE a.principal_id=users.id AND a.principal_type='user' AND a.role_key=$2 AND a.scope_type='platform' AND (a.expires_at IS NULL OR a.expires_at>now())))`,
		search, role, status,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, email, role, disabled, created_at FROM users
		 WHERE ($1 = '' OR name ILIKE '%' || $1 || '%' OR email ILIKE '%' || $1 || '%')
		   AND ($5 = '' OR disabled=($5='disabled'))
 AND ($2 = '' OR role = $2 OR EXISTS (SELECT 1 FROM iam_assignments a WHERE a.principal_id=users.id AND a.principal_type='user' AND a.role_key=$2 AND a.scope_type='platform' AND (a.expires_at IS NULL OR a.expires_at>now())))
		 ORDER BY id DESC LIMIT $3 OFFSET $4`,
		search, role, limit, offset, status,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []AdminUser
	for rows.Next() {
		var u AdminUser
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.Disabled, &u.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, u)
	}
	return out, total, rows.Err()
}

// SetUserDisabled enables or disables an account.
func (r *AuthRepository) SetUserDisabled(ctx context.Context, id int64, disabled bool) (AdminUser, error) {
	var u AdminUser
	err := r.db.QueryRowContext(ctx,
		`UPDATE users SET disabled = $2 WHERE id = $1
		 RETURNING id, name, email, role, disabled, created_at`,
		id, disabled,
	).Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.Disabled, &u.CreatedAt)
	if err != nil {
		return AdminUser{}, ErrNotFound
	}
	return u, nil
}

// ListUsersByIDs loads a batch of users for internal directory lookups.
func (r *AuthRepository) ListUsersByIDs(ctx context.Context, ids []int64) ([]AdminUser, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, email, role, disabled, created_at FROM users WHERE id = ANY($1) ORDER BY id`,
		ids,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AdminUser
	for rows.Next() {
		var u AdminUser
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.Disabled, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// ListAuditLog returns audit entries filtered by actor and action.
func (r *AuthRepository) ListAuditLog(ctx context.Context, actorID int64, action string, limit, offset int) ([]AuditEntry, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, actor_id, action, resource, result, request_id, host(ip), detail, at
		 FROM audit_log
		 WHERE ($1 = 0 OR actor_id = $1) AND ($2 = '' OR action = $2)
		 ORDER BY id DESC LIMIT $3 OFFSET $4`,
		actorID, action, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.ActorID, &e.Action, &e.Resource, &e.Result, &e.RequestID, &e.IP, &e.Detail, &e.At); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
