package repository

import (
	"context"

	"ticketdot/internal/auth/model"
)

type AuthRepository interface {
	CreateUser(user *model.User) error
	GetUserByEmail(email string) (*model.User, error)
	GetUserByID(id string) (*model.User, error)
	GetOperatorIDByUserID(userID string) (string, error)
	CreatePasswordReset(reset *model.PasswordReset) error
	GetValidPasswordReset(token string) (*model.PasswordReset, error)
	MarkPasswordResetUsed(token string) error
	UpdateUserPassword(userID string, passwordHash string) error
	CreateRefreshToken(token *model.RefreshToken) error
	GetRefreshToken(tokenHash string) (*model.RefreshToken, error)
	RotateRefreshToken(tokenHash string) (*model.RefreshToken, error)
	RevokeFamily(familyID string) error
	RevokeRefreshToken(tokenHash string) error
	RevokeUserRefreshTokens(userID string) error
	DeleteExpiredRefreshTokens(ctx context.Context) (int64, error)
}
