package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
)

// HasPlatformAssignment distinguishes an unmigrated legacy account from a user
// whose managed grants exist but have expired. Expiry must not revive legacy admin.
func (r *AuthRepository) HasPlatformAssignment(ctx context.Context, userID int64) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM iam_assignments WHERE principal_type='user' AND principal_id=$1 AND scope_type='platform')`, userID).Scan(&exists)
	return exists, err
}

func writeMutationAudit(ctx context.Context, tx *sql.Tx, actor int64, action, resource, requestID, ip string, detail any) error {
	data, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	var address any
	if parsed := net.ParseIP(ip); parsed != nil {
		address = parsed.String()
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_log(actor_id,action,resource,result,request_id,ip,detail) VALUES($1,$2,$3,'allow',NULLIF($4,''),$5::inet,$6::jsonb)`, actor, action, resource, requestID, address, data)
	return err
}

func (r *AuthRepository) CreateAssignmentAudited(ctx context.Context, a IAMAssignment, requestID, ip string) (IAMAssignment, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return IAMAssignment{}, err
	}
	defer tx.Rollback()
	var exists bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1)`, a.PrincipalID).Scan(&exists); err != nil {
		return IAMAssignment{}, err
	}
	if !exists {
		return IAMAssignment{}, ErrNotFound
	}
	var out IAMAssignment
	err = tx.QueryRowContext(ctx, `INSERT INTO iam_assignments(principal_type,principal_id,role_key,scope_type,scope_id,created_by,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(principal_type,principal_id,role_key,scope_type,scope_id) DO UPDATE SET expires_at=EXCLUDED.expires_at RETURNING id,principal_type,principal_id,role_key,scope_type,scope_id,created_by,created_at,expires_at`, a.PrincipalType, a.PrincipalID, a.RoleKey, a.ScopeType, a.ScopeID, a.CreatedBy, a.ExpiresAt).Scan(&out.ID, &out.PrincipalType, &out.PrincipalID, &out.RoleKey, &out.ScopeType, &out.ScopeID, &out.CreatedBy, &out.CreatedAt, &out.ExpiresAt)
	if err != nil {
		return IAMAssignment{}, err
	}
	if err = writeMutationAudit(ctx, tx, a.CreatedBy, "iam:AssignRole", fmt.Sprintf("hq:user/%d", a.PrincipalID), requestID, ip, map[string]any{"roleKey": a.RoleKey, "scopeType": a.ScopeType, "assignmentId": out.ID}); err != nil {
		return IAMAssignment{}, err
	}
	if err = tx.Commit(); err != nil {
		return IAMAssignment{}, err
	}
	return out, nil
}

func (r *AuthRepository) DeleteAssignmentAudited(ctx context.Context, id, actor int64, requestID, ip string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var userID int64
	var role, scope string
	err = tx.QueryRowContext(ctx, `DELETE FROM iam_assignments WHERE id=$1 RETURNING principal_id,role_key,scope_type`, id).Scan(&userID, &role, &scope)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if role == "PlatformAdmin" && scope == "platform" {
		if _, err = tx.ExecContext(ctx, `UPDATE users SET role='user' WHERE id=$1 AND role='admin' AND NOT EXISTS(SELECT 1 FROM iam_assignments WHERE principal_id=$1 AND principal_type='user' AND scope_type='platform' AND role_key='PlatformAdmin' AND (expires_at IS NULL OR expires_at>now()))`, userID); err != nil {
			return err
		}
	}
	if err = writeMutationAudit(ctx, tx, actor, "iam:RevokeRole", fmt.Sprintf("hq:user/%d", userID), requestID, ip, map[string]any{"roleKey": role, "scopeType": scope, "assignmentId": id}); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *AuthRepository) SetUserDisabledAudited(ctx context.Context, id, actor int64, disabled bool, requestID, ip string) (AdminUser, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return AdminUser{}, err
	}
	defer tx.Rollback()
	var u AdminUser
	err = tx.QueryRowContext(ctx, `UPDATE users SET disabled=$2 WHERE id=$1 RETURNING id,name,email,role,disabled,created_at`, id, disabled).Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.Disabled, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return AdminUser{}, ErrNotFound
	}
	if err != nil {
		return AdminUser{}, err
	}
	if disabled {
		if _, err = tx.ExecContext(ctx, `UPDATE sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE user_id=$1`, id); err != nil {
			return AdminUser{}, err
		}
	}
	if err = writeMutationAudit(ctx, tx, actor, "user:Disable", fmt.Sprintf("hq:user/%d", id), requestID, ip, map[string]bool{"disabled": disabled}); err != nil {
		return AdminUser{}, err
	}
	if err = tx.Commit(); err != nil {
		return AdminUser{}, err
	}
	return u, nil
}
