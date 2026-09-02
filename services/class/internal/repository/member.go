package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/model"
)

// MemberRepository implements model.MemberRepository backed by PostgreSQL.
type MemberRepository struct {
	db *sql.DB
}

func NewMemberRepository(db *sql.DB) *MemberRepository {
	return &MemberRepository{db: db}
}

// Add inserts a new member. Returns ErrConflict if user is already a member.
func (r *MemberRepository) Add(ctx context.Context, classID, userID int64, role string) (*model.ClassMember, error) {
	var m model.ClassMember
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO class_members (class_id, user_id, role)
		VALUES ($1, $2, $3)
		RETURNING id, class_id, user_id, role, joined_at
	`, classID, userID, role).
		Scan(&m.ID, &m.ClassID, &m.UserID, &m.Role, &m.JoinedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrConflict
		}
		return nil, err
	}
	return &m, nil
}

// ListByClass returns all members of a class.
func (r *MemberRepository) ListByClass(ctx context.Context, classID int64) ([]*model.ClassMember, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, class_id, user_id, role, joined_at
		FROM class_members
		WHERE class_id = $1
		ORDER BY joined_at ASC
	`, classID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []*model.ClassMember
	for rows.Next() {
		m := &model.ClassMember{}
		if err := rows.Scan(&m.ID, &m.ClassID, &m.UserID, &m.Role, &m.JoinedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// GetRole returns the role of a user in a class, or empty string if not a member.
func (r *MemberRepository) GetRole(ctx context.Context, classID, userID int64) (string, error) {
	var role string
	err := r.db.QueryRowContext(ctx, `
		SELECT role FROM class_members WHERE class_id = $1 AND user_id = $2
	`, classID, userID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return role, err
}

// UpdateRole changes a member's role.
func (r *MemberRepository) UpdateRole(ctx context.Context, classID, userID int64, role string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE class_members SET role = $1 WHERE class_id = $2 AND user_id = $3
	`, role, classID, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Remove deletes a member from a class.
func (r *MemberRepository) Remove(ctx context.Context, classID, userID int64) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM class_members WHERE class_id = $1 AND user_id = $2
	`, classID, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// CountByClass returns the number of members in a class.
func (r *MemberRepository) CountByClass(ctx context.Context, classID int64) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM class_members WHERE class_id = $1
	`, classID).Scan(&count)
	return count, err
}

// isUniqueViolation checks if the error is a PostgreSQL unique constraint violation.
// Supports pgx, pq, and generic string-based detection.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	// pgx/lib/pq: check the error string for pgcode 23505 or common keywords
	s := err.Error()
	return strings.Contains(s, "23505") || strings.Contains(s, "unique constraint") || strings.Contains(s, "duplicate key")
}
