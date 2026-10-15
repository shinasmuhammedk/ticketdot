package repository

import "ticketdot/internal/auth/model"

type AuthRepository interface {
	CreateUser(user *model.User) error
	GetUserByEmail(email string) (*model.User, error)
	GetUserByID(id string) (*model.User, error)
	GetOperatorIDByUserID(userID string) (string, error)
}
