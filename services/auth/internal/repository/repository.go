package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/model"
)

var ErrNotFound = errors.New("not found")
var ErrEmailTaken = errors.New("email already registered")

type AuthRepository struct {
	db *sql.DB
}

func New(db *sql.DB) *AuthRepository {
	return &AuthRepository{db: db}
}

// CreateUser inserts a new user and returns the created record.
func (r *AuthRepository) CreateUser(ctx context.Context, name, email, passwordHash string) (model.User, error) {
	var u model.User
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO users (name, email, password_hash)
		 VALUES ($1, $2, $3)
		 RETURNING id, name, email, image, role, created_at`,
		name, email, passwordHash,
	).Scan(&u.ID, &u.Name, &u.Email, &u.Image, &u.Role, &u.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return model.User{}, ErrEmailTaken
		}
		return model.User{}, err
	}
	return u, nil
}

// GetUserByEmail returns a user + password_hash for login.
func (r *AuthRepository) GetUserByEmail(ctx context.Context, email string) (model.User, string, error) {
	var u model.User
	var hash string
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, email, image, role, created_at, password_hash
		 FROM users WHERE email = $1`,
		email,
	).Scan(&u.ID, &u.Name, &u.Email, &u.Image, &u.Role, &u.CreatedAt, &hash)
	if err != nil {
		return model.User{}, "", ErrNotFound
	}
	return u, hash, nil
}

// GetUserByID returns a user by primary key.
func (r *AuthRepository) GetUserByID(ctx context.Context, id int64) (model.User, error) {
	var u model.User
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, email, image, role, created_at FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.Name, &u.Email, &u.Image, &u.Role, &u.CreatedAt)
	if err != nil {
		return model.User{}, ErrNotFound
	}
	return u, nil
}

// UpdateProfile updates name and image for a user. P2-AUTH-03.
func (r *AuthRepository) UpdateProfile(ctx context.Context, id int64, name, image string) (model.User, error) {
	var u model.User
	err := r.db.QueryRowContext(ctx,
		`UPDATE users SET name = $1, image = $2
		 WHERE id = $3
		 RETURNING id, name, email, image, role, created_at`,
		name, image, id,
	).Scan(&u.ID, &u.Name, &u.Email, &u.Image, &u.Role, &u.CreatedAt)
	if err != nil {
		return model.User{}, ErrNotFound
	}
	return u, nil
}

// CreateSession stores a hashed token. P2-AUTH-01: explicit TTL.
func (r *AuthRepository) CreateSession(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO sessions (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		userID, tokenHash, expiresAt,
	)
	return err
}

// GetUserByTokenHash looks up the live session and returns the user.
func (r *AuthRepository) GetUserByTokenHash(ctx context.Context, tokenHash string) (model.User, error) {
	var u model.User
	err := r.db.QueryRowContext(ctx,
		`SELECT u.id, u.name, u.email, u.image, u.role, u.created_at
		 FROM sessions s
		 JOIN users u ON u.id = s.user_id
		 WHERE s.token_hash = $1 AND s.expires_at > now()`,
		tokenHash,
	).Scan(&u.ID, &u.Name, &u.Email, &u.Image, &u.Role, &u.CreatedAt)
	if err != nil {
		return model.User{}, ErrNotFound
	}
	return u, nil
}

// DeleteSession removes one session (logout current device). P2-AUTH-01.
func (r *AuthRepository) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM sessions WHERE token_hash = $1`,
		tokenHash,
	)
	return err
}

// DeleteAllSessions removes ALL sessions for a user (logout all devices). P2-AUTH-01.
func (r *AuthRepository) DeleteAllSessions(ctx context.Context, userID int64) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM sessions WHERE user_id = $1`,
		userID,
	)
	return err
}

// PruneExpiredSessions deletes all expired sessions across all users. P2-AUTH-01.
// Safe to call on startup or via a background job.
func (r *AuthRepository) PruneExpiredSessions(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= now()`)
	return err
}
