package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

// P2-AUTH-04: typed sentinel errors mapped to HTTP codes by handler.
var (
	ErrEmailTaken        = errors.New("email already registered")  // 409
	ErrInvalidCredential = errors.New("invalid email or password") // 401
	ErrInvalidSession    = errors.New("invalid or expired session") // 401
	ErrExpiredSession    = errors.New("expired session")            // 401
	ErrRevokedSession    = errors.New("revoked session")            // 401
	ErrDisabledUser      = errors.New("disabled user")               // 401
	ErrForbidden         = errors.New("forbidden")                 // 403
)

var emailRE = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// AuthService holds a Repository interface so it can be tested with a mock. P2-AUTH-05.
type AuthService struct {
	repo       Repository
	sessionTTL time.Duration
}

// New wires up the service with the real repository.
func New(repo *repository.AuthRepository, sessionTTL time.Duration) *AuthService {
	return &AuthService{repo: repo, sessionTTL: sessionTTL}
}

// NewForTest wires up the service with any Repository (e.g. mock). P2-AUTH-05.
func NewForTest(repo Repository, sessionTTL time.Duration) *AuthService {
	return &AuthService{repo: repo, sessionTTL: sessionTTL}
}

// --- P2-AUTH-01: session lifecycle ---

func (s *AuthService) Register(ctx context.Context, input model.RegisterInput) (model.SessionResponse, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))

	if err := validateRegister(input); err != nil {
		return model.SessionResponse{}, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return model.SessionResponse{}, err
	}

	created, err := s.repo.CreateUser(ctx, input.Name, input.Email, string(hash))
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || errors.Is(err, repository.ErrEmailTaken) {
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
	_, u, err := s.verifiedIdentity(ctx, token)
	if err != nil {
		return model.User{}, err
	}
	return u, nil
}

// Logout removes the current session only.
func (s *AuthService) Logout(ctx context.Context, token string) {
	if token != "" {
		_ = s.repo.DeleteSession(ctx, HashToken(token))
	}
}

// LogoutAll removes ALL sessions for the authenticated user. P2-AUTH-01.
func (s *AuthService) LogoutAll(ctx context.Context, token string) error {
	u, err := s.Me(ctx, token)
	if err != nil {
		return ErrInvalidSession
	}
	return s.repo.DeleteAllSessions(ctx, u.ID)
}

// Refresh issues a new token and deletes the old session. P2-AUTH-01.
func (s *AuthService) Refresh(ctx context.Context, token string) (model.SessionResponse, error) {
	u, err := s.Me(ctx, token)
	if err != nil {
		return model.SessionResponse{}, ErrInvalidSession
	}
	_ = s.repo.DeleteSession(ctx, HashToken(token))
	return s.newSession(ctx, u)
}

// --- P2-AUTH-02: token verifier for gateway/study ---

func (s *AuthService) VerifyToken(ctx context.Context, token string) (model.VerifiedIdentity, error) {
	identity, _, err := s.verifiedIdentity(ctx, token)
	return identity, err
}

// --- P2-AUTH-03: user profile ---

func (s *AuthService) GetProfile(ctx context.Context, token string) (model.User, error) {
	return s.Me(ctx, token)
}

func (s *AuthService) UpdateProfile(ctx context.Context, token string, input model.UpdateProfileInput) (model.User, error) {
	u, err := s.Me(ctx, token)
	if err != nil {
		return model.User{}, ErrInvalidSession
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return model.User{}, &ValidationError{Field: "name", Msg: "name is required"}
	}
	return s.repo.UpdateProfile(ctx, u.ID, input.Name, input.Image)
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

func (s *AuthService) verifiedIdentity(ctx context.Context, token string) (model.VerifiedIdentity, model.User, error) {
	if strings.TrimSpace(token) == "" {
		return model.VerifiedIdentity{}, model.User{}, ErrInvalidSession
	}
	u, session, err := s.repo.GetSessionIdentity(ctx, HashToken(token))
	if err != nil {
		return model.VerifiedIdentity{}, model.User{}, ErrInvalidSession
	}
	if u.Disabled {
		return model.VerifiedIdentity{}, model.User{}, ErrDisabledUser
	}
	if session.RevokedAt != nil {
		return model.VerifiedIdentity{}, model.User{}, ErrRevokedSession
	}
	if !session.ExpiresAt.After(time.Now().UTC()) {
		return model.VerifiedIdentity{}, model.User{}, ErrExpiredSession
	}
	return model.VerifiedIdentity{
		UserID: u.ID, Email: u.Email, Name: u.Name, Role: u.Role, ExpiresAt: session.ExpiresAt,
	}, u, nil
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

// --- P2-AUTH-04: validation ---

func validateRegister(input model.RegisterInput) error {
	if input.Name == "" {
		return &ValidationError{Field: "name", Msg: "name is required"}
	}
	if !emailRE.MatchString(input.Email) {
		return &ValidationError{Field: "email", Msg: "valid email is required"}
	}
	if len(input.Password) < 6 {
		return &ValidationError{Field: "password", Msg: "password must be at least 6 characters"}
	}
	return nil
}

// ValidationError carries field-level detail for 422 responses. P2-AUTH-04.
type ValidationError struct {
	Field string
	Msg   string
}

func (e *ValidationError) Error() string { return e.Msg }
