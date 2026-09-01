package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/service"
)

// --- mock repository ---

type mockRepo struct {
	users    map[string]mockUser    // keyed by email
	sessions map[string]mockSession // tokenHash -> session
	nextID   int64
}

type mockUser struct {
	model.User
	passwordHash string
}

type mockSession struct {
	userID    int64
	expiresAt time.Time
	revokedAt *time.Time
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		users:    make(map[string]mockUser),
		sessions: make(map[string]mockSession),
		nextID:   1,
	}
}

func (m *mockRepo) CreateUser(_ context.Context, name, email, hash string) (model.User, error) {
	if _, exists := m.users[email]; exists {
		return model.User{}, errors.New("duplicate key")
	}
	u := model.User{ID: m.nextID, Name: name, Email: email, Role: "user", CreatedAt: time.Now()}
	m.nextID++
	m.users[email] = mockUser{User: u, passwordHash: hash}
	return u, nil
}

func (m *mockRepo) GetUserByEmail(_ context.Context, email string) (model.User, string, error) {
	mu, ok := m.users[email]
	if !ok {
		return model.User{}, "", errors.New("not found")
	}
	return mu.User, mu.passwordHash, nil
}

func (m *mockRepo) GetUserByID(_ context.Context, id int64) (model.User, error) {
	for _, mu := range m.users {
		if mu.User.ID == id {
			return mu.User, nil
		}
	}
	return model.User{}, errors.New("not found")
}

func (m *mockRepo) UpdateProfile(_ context.Context, id int64, name, image string) (model.User, error) {
	for email, mu := range m.users {
		if mu.User.ID == id {
			mu.User.Name = name
			mu.User.Image = image
			m.users[email] = mu
			return mu.User, nil
		}
	}
	return model.User{}, errors.New("not found")
}

func (m *mockRepo) CreateSession(_ context.Context, userID int64, tokenHash string, expiresAt time.Time) error {
	m.sessions[tokenHash] = mockSession{userID: userID, expiresAt: expiresAt}
	return nil
}

func (m *mockRepo) GetSessionIdentity(_ context.Context, tokenHash string) (model.User, model.Session, error) {
	s, ok := m.sessions[tokenHash]
	if !ok {
		return model.User{}, model.Session{}, errors.New("not found")
	}
	u, err := m.GetUserByID(context.Background(), s.userID)
	return u, model.Session{UserID: s.userID, ExpiresAt: s.expiresAt, RevokedAt: s.revokedAt}, err
}

func (m *mockRepo) DeleteSession(_ context.Context, tokenHash string) error {
	if s, ok := m.sessions[tokenHash]; ok {
		now := time.Now().UTC()
		s.revokedAt = &now
		m.sessions[tokenHash] = s
	}
	return nil
}

func (m *mockRepo) DeleteAllSessions(_ context.Context, userID int64) error {
	for hash, session := range m.sessions {
		if session.userID == userID {
			now := time.Now().UTC()
			session.revokedAt = &now
			m.sessions[hash] = session
		}
	}
	return nil
}

func (m *mockRepo) PruneExpiredSessions(_ context.Context) error { return nil }

// --- adapt mock to what service.New expects ---
// service.New takes *repository.AuthRepository, so we need to test via a thin adapter.
// Instead, we test the service behaviour through the exported methods by constructing
// a real service with a real in-memory-like approach: we embed the mock into a wrapper
// that satisfies the repository interface.

// repoAdapter makes mockRepo implement the interface the service uses internally.
// Since service takes a concrete *repository.AuthRepository we can't swap it out directly.
// We therefore test at the HTTP-handler level with a real service wired to a real DB,
// OR we test validateRegister logic separately which is the most bug-prone part.

// For pure unit tests without a DB we test the exported helpers and validation logic.

// TestValidateRegister_MissingName checks name validation. P2-AUTH-05.
func TestValidateRegister_EmptyName(t *testing.T) {
	svc := service.NewForTest(newMockRepo(), 24*time.Hour)
	_, err := svc.Register(context.Background(), model.RegisterInput{
		Name:     "",
		Email:    "test@example.com",
		Password: "secret123",
	})
	if err == nil {
		t.Fatal("expected error for empty name, got nil")
	}
}

func TestValidateRegister_BadEmail(t *testing.T) {
	svc := service.NewForTest(newMockRepo(), 24*time.Hour)
	_, err := svc.Register(context.Background(), model.RegisterInput{
		Name:     "Test",
		Email:    "not-an-email",
		Password: "secret123",
	})
	if err == nil {
		t.Fatal("expected error for bad email, got nil")
	}
}

func TestValidateRegister_ShortPassword(t *testing.T) {
	svc := service.NewForTest(newMockRepo(), 24*time.Hour)
	_, err := svc.Register(context.Background(), model.RegisterInput{
		Name:     "Test",
		Email:    "test@example.com",
		Password: "123",
	})
	if err == nil {
		t.Fatal("expected error for short password, got nil")
	}
}

func TestRegisterLogin_HappyPath(t *testing.T) {
	repo := newMockRepo()
	svc := service.NewForTest(repo, 24*time.Hour)
	ctx := context.Background()

	// Register
	resp, err := svc.Register(ctx, model.RegisterInput{
		Name:     "Nguyen Van A",
		Email:    "a@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if !resp.Authenticated {
		t.Error("expected authenticated=true after register")
	}
	if resp.Token == "" {
		t.Error("expected token after register")
	}

	// Me
	u, err := svc.Me(ctx, resp.Token)
	if err != nil {
		t.Fatalf("me failed: %v", err)
	}
	if u.Email != "a@example.com" {
		t.Errorf("got email %q, want a@example.com", u.Email)
	}

	// Login
	loginResp, err := svc.Login(ctx, model.LoginInput{
		Email:    "a@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if loginResp.Token == "" {
		t.Error("expected token after login")
	}

	// Logout
	svc.Logout(ctx, loginResp.Token)
	_, err = svc.Me(ctx, loginResp.Token)
	if err == nil {
		t.Error("expected error after logout, got nil")
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	repo := newMockRepo()
	svc := service.NewForTest(repo, 24*time.Hour)
	ctx := context.Background()

	input := model.RegisterInput{Name: "A", Email: "dup@example.com", Password: "password123"}
	if _, err := svc.Register(ctx, input); err != nil {
		t.Fatalf("first register failed: %v", err)
	}
	_, err := svc.Register(ctx, input)
	if !errors.Is(err, service.ErrEmailTaken) {
		t.Fatalf("expected ErrEmailTaken, got %v", err)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	repo := newMockRepo()
	svc := service.NewForTest(repo, 24*time.Hour)
	ctx := context.Background()

	_, err := svc.Register(ctx, model.RegisterInput{Name: "B", Email: "b@example.com", Password: "correct"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Login(ctx, model.LoginInput{Email: "b@example.com", Password: "wrong"})
	if !errors.Is(err, service.ErrInvalidCredential) {
		t.Fatalf("expected ErrInvalidCredential, got %v", err)
	}
}

func TestRefresh_OldTokenInvalid(t *testing.T) {
	repo := newMockRepo()
	svc := service.NewForTest(repo, 24*time.Hour)
	ctx := context.Background()

	resp, err := svc.Register(ctx, model.RegisterInput{Name: "C", Email: "c@example.com", Password: "pass123"})
	if err != nil {
		t.Fatal(err)
	}
	oldToken := resp.Token

	newResp, err := svc.Refresh(ctx, oldToken)
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if newResp.Token == oldToken {
		t.Error("expected new token after refresh")
	}
	// Old token should now be invalid
	_, err = svc.Me(ctx, oldToken)
	if err == nil {
		t.Error("old token should be invalid after refresh")
	}
}

func TestLogoutAll(t *testing.T) {
	repo := newMockRepo()
	svc := service.NewForTest(repo, 24*time.Hour)
	ctx := context.Background()

	resp1, _ := svc.Register(ctx, model.RegisterInput{Name: "D", Email: "d@example.com", Password: "pass123"})
	resp2, _ := svc.Login(ctx, model.LoginInput{Email: "d@example.com", Password: "pass123"})

	// LogoutAll using first token
	if err := svc.LogoutAll(ctx, resp1.Token); err != nil {
		t.Fatalf("logout all failed: %v", err)
	}
	// Both sessions should be gone
	if _, err := svc.Me(ctx, resp1.Token); err == nil {
		t.Error("token1 should be invalid after logout-all")
	}
	if _, err := svc.Me(ctx, resp2.Token); err == nil {
		t.Error("token2 should be invalid after logout-all")
	}
}

func TestUpdateProfile(t *testing.T) {
	repo := newMockRepo()
	svc := service.NewForTest(repo, 24*time.Hour)
	ctx := context.Background()

	resp, _ := svc.Register(ctx, model.RegisterInput{Name: "E", Email: "e@example.com", Password: "pass123"})

	updated, err := svc.UpdateProfile(ctx, resp.Token, model.UpdateProfileInput{
		Name:  "E Updated",
		Image: "https://example.com/avatar.jpg",
	})
	if err != nil {
		t.Fatalf("update profile failed: %v", err)
	}
	if updated.Name != "E Updated" {
		t.Errorf("expected name 'E Updated', got %q", updated.Name)
	}
	if updated.Image != "https://example.com/avatar.jpg" {
		t.Errorf("expected image URL, got %q", updated.Image)
	}
}

func TestHashToken_Deterministic(t *testing.T) {
	h1 := service.HashToken("abc123")
	h2 := service.HashToken("abc123")
	if h1 != h2 {
		t.Error("HashToken should be deterministic")
	}
	if service.HashToken("abc123") == service.HashToken("abc124") {
		t.Error("different tokens should produce different hashes")
	}
}

func TestVerifyTokenRejectsMissingInvalidExpiredRevokedAndDisabled(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*mockRepo, string)
		token string
		want  error
	}{
		{name: "missing", token: "", want: service.ErrInvalidSession},
		{name: "invalid", token: "not-issued", want: service.ErrInvalidSession},
		{name: "expired", token: "issued", setup: func(repo *mockRepo, hash string) {
			s := repo.sessions[hash]
			s.expiresAt = time.Now().UTC().Add(-time.Minute)
			repo.sessions[hash] = s
		}, want: service.ErrExpiredSession},
		{name: "revoked", token: "issued", setup: func(repo *mockRepo, hash string) {
			s := repo.sessions[hash]
			now := time.Now().UTC()
			s.revokedAt = &now
			repo.sessions[hash] = s
		}, want: service.ErrRevokedSession},
		{name: "disabled", token: "issued", setup: func(repo *mockRepo, _ string) {
			u := repo.users["verify@example.com"]
			u.Disabled = true
			repo.users["verify@example.com"] = u
		}, want: service.ErrDisabledUser},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepo()
			svc := service.NewForTest(repo, time.Hour)
			resp, err := svc.Register(context.Background(), model.RegisterInput{
				Name: "Verify", Email: "verify@example.com", Password: "password123",
			})
			if err != nil {
				t.Fatal(err)
			}
			if tt.setup != nil {
				tt.setup(repo, service.HashToken(resp.Token))
			}
			token := tt.token
			if token == "issued" {
				token = resp.Token
			}
			_, err = svc.VerifyToken(context.Background(), token)
			if !errors.Is(err, tt.want) {
				t.Fatalf("VerifyToken() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestVerifyTokenReturnsCanonicalIdentityAndExpiry(t *testing.T) {
	repo := newMockRepo()
	svc := service.NewForTest(repo, time.Hour)
	resp, err := svc.Register(context.Background(), model.RegisterInput{
		Name: "Canonical", Email: "canonical@example.com", Password: "password123",
	})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := svc.VerifyToken(context.Background(), resp.Token)
	if err != nil {
		t.Fatal(err)
	}
	if identity.UserID != resp.User.ID || identity.Email != resp.User.Email {
		t.Fatalf("unexpected identity: %+v", identity)
	}
	if identity.ExpiresAt.IsZero() || !identity.ExpiresAt.Equal(resp.ExpiresAt) {
		t.Fatalf("expiry = %v, want %v", identity.ExpiresAt, resp.ExpiresAt)
	}
}
