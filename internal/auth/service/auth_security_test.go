package service_test

import (
	"context"
	"database/sql"
	"os"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"ticketdot/internal/auth/model"
	"ticketdot/internal/auth/service"
)

type memoryAuthRepo struct {
	mu            sync.Mutex
	users         map[string]*model.User
	resets        map[string]*model.PasswordReset
	refreshTokens map[string]*model.RefreshToken
}

func newMemoryAuthRepo() *memoryAuthRepo {
	return &memoryAuthRepo{
		users:         make(map[string]*model.User),
		resets:        make(map[string]*model.PasswordReset),
		refreshTokens: make(map[string]*model.RefreshToken),
	}
}

func (m *memoryAuthRepo) CreateUser(u *model.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[u.Email] = u
	return nil
}

func (m *memoryAuthRepo) GetUserByEmail(email string) (*model.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[email]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return u, nil
}

func (m *memoryAuthRepo) GetUserByID(id string) (*model.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (m *memoryAuthRepo) GetOperatorIDByUserID(userID string) (string, error) {
	return "", nil
}

func (m *memoryAuthRepo) CreatePasswordReset(reset *model.PasswordReset) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resets[reset.Token] = reset
	return nil
}

func (m *memoryAuthRepo) GetValidPasswordReset(token string) (*model.PasswordReset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.resets[token]
	if !ok || r.Used || r.ExpiresAt.Before(time.Now()) {
		return nil, sql.ErrNoRows
	}
	return r, nil
}

func (m *memoryAuthRepo) MarkPasswordResetUsed(token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.resets[token]; ok {
		r.Used = true
	}
	return nil
}

func (m *memoryAuthRepo) UpdateUserPassword(userID string, passwordHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if u.ID == userID {
			u.PasswordHash = passwordHash
			return nil
		}
	}
	return sql.ErrNoRows
}

func (m *memoryAuthRepo) CreateRefreshToken(token *model.RefreshToken) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refreshTokens[token.TokenHash] = token
	return nil
}

func (m *memoryAuthRepo) GetRefreshToken(tokenHash string) (*model.RefreshToken, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.refreshTokens[tokenHash]
	if !ok {
		return nil, sql.ErrNoRows
	}
	copied := *t
	return &copied, nil
}

func (m *memoryAuthRepo) RotateRefreshToken(tokenHash string) (*model.RefreshToken, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.refreshTokens[tokenHash]
	if !ok || t.RevokedAt != nil || time.Now().After(t.ExpiresAt) {
		return nil, sql.ErrNoRows
	}

	now := time.Now()
	t.RevokedAt = &now
	copied := *t
	return &copied, nil
}

func (m *memoryAuthRepo) RevokeFamily(familyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for _, t := range m.refreshTokens {
		if t.FamilyID == familyID && t.RevokedAt == nil {
			t.RevokedAt = &now
		}
	}
	return nil
}

func (m *memoryAuthRepo) RevokeRefreshToken(tokenHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if t, ok := m.refreshTokens[tokenHash]; ok && t.RevokedAt == nil {
		now := time.Now()
		t.RevokedAt = &now
	}
	return nil
}

func (m *memoryAuthRepo) RevokeUserRefreshTokens(userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for _, t := range m.refreshTokens {
		if t.UserID == userID && t.RevokedAt == nil {
			t.RevokedAt = &now
		}
	}
	return nil
}

func (m *memoryAuthRepo) DeleteExpiredRefreshTokens(ctx context.Context) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	var deleted int64
	for hash, t := range m.refreshTokens {
		if t.ExpiresAt.Before(cutoff) {
			delete(m.refreshTokens, hash)
			deleted++
		}
	}
	return deleted, nil
}

func TestRefreshTokenRotation(t *testing.T) {
	os.Setenv("JWT_SECRET", "super-secret-key-that-is-at-least-32-chars-long")

	repo := newMemoryAuthRepo()
	svc := service.NewAuthService(repo)

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), 12)
	user := &model.User{
		ID:           "user-rot-1",
		Email:        "rotation@example.com",
		PasswordHash: string(hashedPassword),
		Role:         "passenger",
	}
	repo.users[user.Email] = user

	// 1. Login generates access token and refresh token
	loginRes, err := svc.Login(service.LoginRequest{
		Email:    "rotation@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if loginRes.RefreshToken == "" {
		t.Fatal("expected refresh token on login")
	}

	firstRefreshToken := loginRes.RefreshToken

	// 2. Refresh token rotation: Use refresh token to get a new pair
	refreshRes, err := svc.Refresh(service.RefreshRequest{
		RefreshToken: firstRefreshToken,
	})
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if refreshRes.RefreshToken == "" || refreshRes.RefreshToken == firstRefreshToken {
		t.Fatal("expected newly rotated refresh token distinct from the first")
	}

	// 3. Old refresh token MUST be rejected after use
	_, err = svc.Refresh(service.RefreshRequest{
		RefreshToken: firstRefreshToken,
	})
	if err == nil {
		t.Fatal("expected old refresh token to be rejected after rotation, got nil")
	}
}

func TestRefreshTokenReuseDetectionRevokesFamily(t *testing.T) {
	os.Setenv("JWT_SECRET", "super-secret-key-that-is-at-least-32-chars-long")

	repo := newMemoryAuthRepo()
	svc := service.NewAuthService(repo)

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), 12)
	user := &model.User{
		ID:           "user-reuse-1",
		Email:        "reuse@example.com",
		PasswordHash: string(hashedPassword),
		Role:         "passenger",
	}
	repo.users[user.Email] = user

	// 1. Initial login -> Token A
	loginRes, err := svc.Login(service.LoginRequest{
		Email:    "reuse@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	tokenA := loginRes.RefreshToken

	// 2. Legitimate rotation: Token A -> Token B
	rotRes1, err := svc.Refresh(service.RefreshRequest{RefreshToken: tokenA})
	if err != nil {
		t.Fatalf("first refresh failed: %v", err)
	}
	tokenB := rotRes1.RefreshToken

	// 3. Legitimate rotation: Token B -> Token C
	rotRes2, err := svc.Refresh(service.RefreshRequest{RefreshToken: tokenB})
	if err != nil {
		t.Fatalf("second refresh failed: %v", err)
	}
	tokenC := rotRes2.RefreshToken

	// 4. Attacker attempts to reuse already rotated Token A!
	_, err = svc.Refresh(service.RefreshRequest{RefreshToken: tokenA})
	if err == nil {
		t.Fatal("expected reuse of Token A to fail, got nil")
	}

	// 5. Token reuse MUST revoke the entire family!
	// Therefore, even the legitimate active Token C must now be rejected!
	_, err = svc.Refresh(service.RefreshRequest{RefreshToken: tokenC})
	if err == nil {
		t.Fatal("expected legitimate Token C to be revoked following Token A reuse detection, got nil")
	}
}

func TestConcurrentRefreshTokenRotation(t *testing.T) {
	os.Setenv("JWT_SECRET", "super-secret-key-that-is-at-least-32-chars-long")

	repo := newMemoryAuthRepo()
	svc := service.NewAuthService(repo)

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), 12)
	user := &model.User{
		ID:           "user-concurrent-1",
		Email:        "concurrent@example.com",
		PasswordHash: string(hashedPassword),
		Role:         "passenger",
	}
	repo.users[user.Email] = user

	loginRes, err := svc.Login(service.LoginRequest{
		Email:    "concurrent@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	token := loginRes.RefreshToken

	// Attempt concurrent refreshes on the same token
	const concurrency = 10
	var wg sync.WaitGroup
	var successCount int
	var mu sync.Mutex

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, refErr := svc.Refresh(service.RefreshRequest{RefreshToken: token})
			if refErr == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if successCount != 1 {
		t.Fatalf("expected exactly 1 concurrent refresh to succeed, but got %d", successCount)
	}
}

func TestLogoutRevokesRefreshToken(t *testing.T) {
	os.Setenv("JWT_SECRET", "super-secret-key-that-is-at-least-32-chars-long")

	repo := newMemoryAuthRepo()
	svc := service.NewAuthService(repo)

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), 12)
	user := &model.User{
		ID:           "user-logout-1",
		Email:        "logout@example.com",
		PasswordHash: string(hashedPassword),
		Role:         "passenger",
	}
	repo.users[user.Email] = user

	loginRes, err := svc.Login(service.LoginRequest{
		Email:    "logout@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	// Logout with the refresh token
	err = svc.Logout(service.LogoutRequest{
		RefreshToken: loginRes.RefreshToken,
	})
	if err != nil {
		t.Fatalf("logout failed: %v", err)
	}

	// Refresh token must now be revoked and rejected
	_, err = svc.Refresh(service.RefreshRequest{
		RefreshToken: loginRes.RefreshToken,
	})
	if err == nil {
		t.Fatal("expected revoked refresh token to be rejected, got nil")
	}
}

func TestPasswordResetRevokesAllRefreshTokens(t *testing.T) {
	os.Setenv("JWT_SECRET", "super-secret-key-that-is-at-least-32-chars-long")

	repo := newMemoryAuthRepo()
	svc := service.NewAuthService(repo)

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("oldpassword123"), 12)
	user := &model.User{
		ID:           "user-pwreset-1",
		Email:        "resetpw@example.com",
		PasswordHash: string(hashedPassword),
		Role:         "passenger",
	}
	repo.users[user.Email] = user

	// Login and obtain refresh token
	loginRes, err := svc.Login(service.LoginRequest{
		Email:    "resetpw@example.com",
		Password: "oldpassword123",
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	// Request password reset
	forgotRes, err := svc.ForgotPassword(service.ForgotPasswordRequest{
		Email: "resetpw@example.com",
	})
	if err != nil {
		t.Fatalf("forgot password failed: %v", err)
	}

	// Reset password
	err = svc.ResetPassword(service.ResetPasswordRequest{
		Token:       forgotRes.ResetToken,
		NewPassword: "newpassword123",
	})
	if err != nil {
		t.Fatalf("reset password failed: %v", err)
	}

	// Previously issued refresh token must now be revoked
	_, err = svc.Refresh(service.RefreshRequest{
		RefreshToken: loginRes.RefreshToken,
	})
	if err == nil {
		t.Fatal("expected refresh token to be revoked following password reset, got nil")
	}
}

func TestPasswordStrengthValidation(t *testing.T) {
	repo := newMemoryAuthRepo()
	svc := service.NewAuthService(repo)

	// Password shorter than 8 characters must fail
	_, err := svc.Signup(service.SignupRequest{
		Name:     "Short PW",
		Email:    "short@example.com",
		Phone:    "1234567890",
		Password: "short1",
	})
	if err == nil {
		t.Fatal("expected password under 8 chars to fail, got nil")
	}

	// Password equal to email must fail
	_, err = svc.Signup(service.SignupRequest{
		Name:     "Equal PW",
		Email:    "testuser@example.com",
		Phone:    "1234567890",
		Password: "testuser@example.com",
	})
	if err == nil {
		t.Fatal("expected password equal to email to fail, got nil")
	}
}
