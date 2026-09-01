package model

import "time"

type User struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Image     string    `json:"image"`
	Role      string    `json:"role"`
	Disabled  bool      `json:"-"`
	CreatedAt time.Time `json:"createdAt"`
}

type Session struct {
	ID        int64      `json:"id"`
	UserID    int64      `json:"userId"`
	TokenHash string     `json:"-"`
	ExpiresAt time.Time  `json:"expiresAt"`
	RevokedAt *time.Time `json:"-"`
	CreatedAt time.Time  `json:"createdAt"`
}

// VerifiedIdentity is the canonical identity returned to trusted services.
// It deliberately excludes the raw token and token hash.
type VerifiedIdentity struct {
	UserID    int64     `json:"userId"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type RegisterInput struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// UpdateProfileInput – P2-AUTH-03
type UpdateProfileInput struct {
	Name  string `json:"name"`
	Image string `json:"image"`
}

type SessionResponse struct {
	Authenticated bool      `json:"authenticated"`
	Token         string    `json:"token"`
	ExpiresAt     time.Time `json:"expiresAt"`
	User          User      `json:"user"`
}
