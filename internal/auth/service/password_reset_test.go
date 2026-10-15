package service

import (
	"database/sql"
	"testing"
	"time"

	"ticketdot/internal/auth/model"
)

type mockAuthRepo struct {
	users   map[string]*model.User
	resets  map[string]*model.PasswordReset
}

func newMockAuthRepo() *mockAuthRepo {
	return &mockAuthRepo{
		users:  make(map[string]*model.User),
		resets: make(map[string]*model.PasswordReset),
	}
}

func (m *mockAuthRepo) CreateUser(u *model.User) error {
	m.users[u.Email] = u
	return nil
}

func (m *mockAuthRepo) GetUserByEmail(email string) (*model.User, error) {
	u, ok := m.users[email]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return u, nil
}

func (m *mockAuthRepo) GetUserByID(id string) (*model.User, error) {
	for _, u := range m.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (m *mockAuthRepo) GetOperatorIDByUserID(userID string) (string, error) {
	return "", nil
}

func (m *mockAuthRepo) CreatePasswordReset(reset *model.PasswordReset) error {
	m.resets[reset.Token] = reset
	return nil
}

func (m *mockAuthRepo) GetValidPasswordReset(token string) (*model.PasswordReset, error) {
	r, ok := m.resets[token]
	if !ok || r.Used || r.ExpiresAt.Before(time.Now()) {
		return nil, sql.ErrNoRows
	}
	return r, nil
}

func (m *mockAuthRepo) MarkPasswordResetUsed(token string) error {
	if r, ok := m.resets[token]; ok {
		r.Used = true
	}
	return nil
}

func (m *mockAuthRepo) UpdateUserPassword(userID string, passwordHash string) error {
	for _, u := range m.users {
		if u.ID == userID {
			u.PasswordHash = passwordHash
			return nil
		}
	}
	return sql.ErrNoRows
}

func TestForgotPasswordAndResetPassword(t *testing.T) {
	repo := newMockAuthRepo()
	svc := NewAuthService(repo)

	// Setup user
	repo.users["alice@example.com"] = &model.User{
		ID:           "user-alice",
		Email:        "alice@example.com",
		PasswordHash: "initial-hash",
	}

	// 1. ForgotPassword
	res, err := svc.ForgotPassword(ForgotPasswordRequest{
		Email: "alice@example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.ResetToken == "" {
		t.Fatalf("expected reset_token to be non-empty")
	}

	// 2. ResetPassword with short password should fail
	err = svc.ResetPassword(ResetPasswordRequest{
		Token:       res.ResetToken,
		NewPassword: "123",
	})
	if err == nil {
		t.Fatalf("expected error for short password, got nil")
	}

	// 3. ResetPassword with valid password should succeed
	err = svc.ResetPassword(ResetPasswordRequest{
		Token:       res.ResetToken,
		NewPassword: "newsecurepassword123",
	})
	if err != nil {
		t.Fatalf("failed to reset password: %v", err)
	}

	// Verify password hash changed
	if repo.users["alice@example.com"].PasswordHash == "initial-hash" {
		t.Fatalf("expected password hash to be updated")
	}

	// 4. Reusing token should fail
	err = svc.ResetPassword(ResetPasswordRequest{
		Token:       res.ResetToken,
		NewPassword: "anotherpassword123",
	})
	if err == nil {
		t.Fatalf("expected token reuse to be rejected, got nil")
	}
}
