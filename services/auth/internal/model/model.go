package model

import "time"

type User struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Image     string    `json:"image"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

type Session struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"userId"`
	TokenHash string    `json:"-"`
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
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
