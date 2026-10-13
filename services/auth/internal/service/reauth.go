package service

import (
	"context"
	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/model"
	"golang.org/x/crypto/bcrypt"
)

// ConfirmPassword verifies the current session and its account password without creating a session.
func (s *AuthService) ConfirmPassword(ctx context.Context, token, password string) (model.User, error) {
	u, err := s.Me(ctx, token)
	if err != nil {
		return model.User{}, err
	}
	_, hash, err := s.repo.GetUserByEmail(ctx, u.Email)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return model.User{}, ErrInvalidCredential
	}
	return u, nil
}
