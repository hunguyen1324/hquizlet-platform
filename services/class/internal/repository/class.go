package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/model"
)

// inviteCodeAlphabet excludes confusing chars (0/O, 1/I/L).
const inviteCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// ClassRepository implements model.ClassRepository backed by PostgreSQL.
type ClassRepository struct {
	db *sql.DB
}

func NewClassRepository(db *sql.DB) *ClassRepository {
	return &ClassRepository{db: db}
}

// GenerateInviteCode produces an 8-character cryptographically random code.
func GenerateInviteCode() (string, error) {
	const codeLen = 8
	buf := make([]byte, codeLen)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate invite code: %w", err)
	}
	for i := range buf {
		buf[i] = inviteCodeAlphabet[int(buf[i])%len(inviteCodeAlphabet)]
	}
	return string(buf), nil
}

// Create inserts a new class and returns the persisted record.
func (r *ClassRepository) Create(ctx context.Context, ownerID int64, input model.CreateClassInput) (*model.Class, error) {
	code, err := GenerateInviteCode()
	if err != nil {
		return nil, err
	}

	var c model.Class
	err = r.db.QueryRowContext(ctx, `
		INSERT INTO classes (owner_user_id, name, description, invite_code, max_members)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, owner_user_id, name, description, invite_code, max_members, created_at, updated_at
	`, ownerID, input.Name, input.Description, code, input.MaxMembers).
		Scan(&c.ID, &c.OwnerUserID, &c.Name, &c.Description, &c.InviteCode, &c.MaxMembers, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// GetByID returns a class by ID.
func (r *ClassRepository) GetByID(ctx context.Context, classID int64) (*model.Class, error) {
	var c model.Class
	err := r.db.QueryRowContext(ctx, `
		SELECT id, owner_user_id, name, description, invite_code, max_members, created_at, updated_at
		FROM classes WHERE id = $1
	`, classID).
		Scan(&c.ID, &c.OwnerUserID, &c.Name, &c.Description, &c.InviteCode, &c.MaxMembers, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

// GetByInviteCode returns a class by its invite code.
func (r *ClassRepository) GetByInviteCode(ctx context.Context, code string) (*model.Class, error) {
	var c model.Class
	err := r.db.QueryRowContext(ctx, `
		SELECT id, owner_user_id, name, description, invite_code, max_members, created_at, updated_at
		FROM classes WHERE invite_code = $1
	`, code).
		Scan(&c.ID, &c.OwnerUserID, &c.Name, &c.Description, &c.InviteCode, &c.MaxMembers, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

// ListByUserID returns class summaries for all classes the user is a member of.
func (r *ClassRepository) ListByUserID(ctx context.Context, userID int64) ([]*model.ClassSummary, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.name, c.description, c.invite_code,
			(SELECT COUNT(*) FROM class_members WHERE class_id = c.id) AS member_count,
			(SELECT COUNT(*) FROM class_study_sets WHERE class_id = c.id) AS study_set_count,
			COALESCE(cm.role, 'owner') AS my_role,
			c.created_at, c.updated_at
		FROM classes c
		LEFT JOIN class_members cm ON cm.class_id = c.id AND cm.user_id = $1
		WHERE c.owner_user_id = $1 OR cm.user_id IS NOT NULL
		ORDER BY c.updated_at DESC, c.id DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []*model.ClassSummary
	for rows.Next() {
		s := &model.ClassSummary{}
		if err := rows.Scan(&s.ID, &s.Name, &s.Description, &s.InviteCode, &s.MemberCount, &s.StudySetCount, &s.MyRole, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		summaries = append(summaries, s)
	}
	return summaries, rows.Err()
}

// Update modifies a class and returns the updated record.
func (r *ClassRepository) Update(ctx context.Context, classID int64, input model.UpdateClassInput) (*model.Class, error) {
	var c model.Class
	err := r.db.QueryRowContext(ctx, `
		UPDATE classes
		SET name = $1, description = $2, updated_at = NOW()
		WHERE id = $3
		RETURNING id, owner_user_id, name, description, invite_code, max_members, created_at, updated_at
	`, input.Name, input.Description, classID).
		Scan(&c.ID, &c.OwnerUserID, &c.Name, &c.Description, &c.InviteCode, &c.MaxMembers, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

// Delete removes a class (cascades to members, study sets, activity events).
func (r *ClassRepository) Delete(ctx context.Context, classID int64) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM classes WHERE id = $1", classID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ResetInviteCode generates a new invite code for a class.
func (r *ClassRepository) ResetInviteCode(ctx context.Context, classID int64, newCode string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE classes SET invite_code = $1, updated_at = NOW() WHERE id = $2
	`, newCode, classID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// GenerateUniqueInviteCode tries up to 5 times to generate a unique code.
func (r *ClassRepository) GenerateUniqueInviteCode(ctx context.Context) (string, error) {
	const maxRetries = 5
	for i := 0; i < maxRetries; i++ {
		code, err := GenerateInviteCode()
		if err != nil {
			return "", err
		}
		var exists bool
		err = r.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM classes WHERE invite_code = $1)", code).Scan(&exists)
		if err != nil {
			return "", err
		}
		if !exists {
			return code, nil
		}
	}
	return "", fmt.Errorf("failed to generate unique invite code after %d attempts", maxRetries)
}

// IsNameBlank checks if name is empty after trimming.
func IsNameBlank(name string) bool {
	return strings.TrimSpace(name) == ""
}
