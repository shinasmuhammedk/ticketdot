package cleanup_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"ticketdot/internal/auth/cleanup"
	"ticketdot/internal/auth/model"
)

type mockCleanupRepo struct {
	mu           sync.Mutex
	deletedCount int64
	called       bool
}

func (m *mockCleanupRepo) CreateUser(user *model.User) error                        { return nil }
func (m *mockCleanupRepo) GetUserByEmail(email string) (*model.User, error)         { return nil, nil }
func (m *mockCleanupRepo) GetUserByID(id string) (*model.User, error)               { return nil, nil }
func (m *mockCleanupRepo) GetOperatorIDByUserID(userID string) (string, error)      { return "", nil }
func (m *mockCleanupRepo) CreatePasswordReset(reset *model.PasswordReset) error     { return nil }
func (m *mockCleanupRepo) GetValidPasswordReset(token string) (*model.PasswordReset, error) {
	return nil, nil
}
func (m *mockCleanupRepo) MarkPasswordResetUsed(token string) error                 { return nil }
func (m *mockCleanupRepo) UpdateUserPassword(userID string, passwordHash string) error {
	return nil
}
func (m *mockCleanupRepo) CreateRefreshToken(token *model.RefreshToken) error       { return nil }
func (m *mockCleanupRepo) GetRefreshToken(tokenHash string) (*model.RefreshToken, error) {
	return nil, nil
}
func (m *mockCleanupRepo) RotateRefreshToken(tokenHash string) (*model.RefreshToken, error) {
	return nil, nil
}
func (m *mockCleanupRepo) RevokeFamily(familyID string) error                       { return nil }
func (m *mockCleanupRepo) RevokeRefreshToken(tokenHash string) error                { return nil }
func (m *mockCleanupRepo) RevokeUserRefreshTokens(userID string) error              { return nil }

func (m *mockCleanupRepo) DeleteExpiredRefreshTokens(ctx context.Context) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.called = true
	m.deletedCount += 5
	return 5, nil
}

func (m *mockCleanupRepo) wasCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.called
}

func TestStartRefreshTokenCleaner_GracefulStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	repo := &mockCleanupRepo{}

	cleanup.StartRefreshTokenCleaner(ctx, repo, 10*time.Millisecond)

	time.Sleep(30 * time.Millisecond)
	cancel()
	time.Sleep(20 * time.Millisecond)

	if !repo.wasCalled() {
		t.Fatal("expected DeleteExpiredRefreshTokens to have been called")
	}
}
