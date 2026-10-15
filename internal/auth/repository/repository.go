package repository

import "ticketdot/internal/auth/model"

type AuthRepository interface {
	CreateUser(user *model.User) error
	GetUserByEmail(email string) (*model.User, error)
	GetUserByID(id string) (*model.User, error)
	GetOperatorIDByUserID(userID string) (string, error)
	CreatePasswordReset(reset *model.PasswordReset) error
	GetValidPasswordReset(token string) (*model.PasswordReset, error)
	MarkPasswordResetUsed(token string) error
	UpdateUserPassword(userID string, passwordHash string) error
}
