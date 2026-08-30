package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEmailTaken        = errors.New("email already registered")
	ErrInvalidCredential = errors.New("invalid email or password")
	ErrInvalidSession    = errors.New("invalid or expired session")
)

type AuthService struct {
	repo       *repository.AuthRepository
	sessionTTL time.Duration
}

func New(repo *repository.AuthRepository, sessionTTL time.Duration) *AuthService {
	return &AuthService{repo: repo, sessionTTL: sessionTTL}
}

func (s *AuthService) Register(ctx context.Context, input model.RegisterInput) (model.SessionResponse, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))

	if input.Name == "" {
		return model.SessionResponse{}, errors.New("name is required")
	}
	if !strings.Contains(input.Email, "@") {
		return model.SessionResponse{}, errors.New("valid email is required")
	}
	if len(input.Password) < 6 {
		return model.SessionResponse{}, errors.New("password must be at least 6 characters")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return model.SessionResponse{}, err
	}

	created, err := s.repo.CreateUser(ctx, input.Name, input.Email, string(hash))
	if err != nil {
		if errors.Is(err, repository.ErrEmailTaken) {
			return model.SessionResponse{}, ErrEmailTaken
		}
		return model.SessionResponse{}, err
	}
	return s.newSession(ctx, created)
}

func (s *AuthService) Login(ctx context.Context, input model.LoginInput) (model.SessionResponse, error) {
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))

	u, hash, err := s.repo.GetUserByEmail(ctx, input.Email)
	if err != nil {
		return model.SessionResponse{}, ErrInvalidCredential
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(input.Password)); err != nil {
		return model.SessionResponse{}, ErrInvalidCredential
	}
	return s.newSession(ctx, u)
}

func (s *AuthService) Me(ctx context.Context, token string) (model.User, error) {
	if token == "" {
		return model.User{}, ErrInvalidSession
	}
	u, err := s.repo.GetUserByTokenHash(ctx, HashToken(token))
	if err != nil {
		return model.User{}, ErrInvalidSession
	}
	return u, nil
}

func (s *AuthService) Logout(ctx context.Context, token string) {
	if token != "" {
		_ = s.repo.DeleteSession(ctx, HashToken(token))
	}
}

func (s *AuthService) Refresh(ctx context.Context, token string) (model.SessionResponse, error) {
	u, err := s.Me(ctx, token)
	if err != nil {
		return model.SessionResponse{}, ErrInvalidSession
	}
	return s.newSession(ctx, u)
}

// VerifyToken is a lightweight helper for other services to validate a bearer token.
// Returns the authenticated User or an error.
func (s *AuthService) VerifyToken(ctx context.Context, token string) (model.User, error) {
	return s.Me(ctx, token)
}

// --- helpers ---

func (s *AuthService) newSession(ctx context.Context, u model.User) (model.SessionResponse, error) {
	token, err := randomToken()
	if err != nil {
		return model.SessionResponse{}, err
	}
	expiresAt := time.Now().UTC().Add(s.sessionTTL)
	if err := s.repo.CreateSession(ctx, u.ID, HashToken(token), expiresAt); err != nil {
		return model.SessionResponse{}, err
	}
	return model.SessionResponse{
		Authenticated: true,
		Token:         token,
		ExpiresAt:     expiresAt,
		User:          u,
	}, nil
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
