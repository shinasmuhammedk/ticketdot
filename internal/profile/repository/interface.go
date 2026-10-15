package repository

import "ticketdot/internal/profile/model"

type Repository interface {
	GetProfile(userID string) (*model.ProfileResponse, error)
	UpdateProfile(userID string, req *model.UpdateProfileRequest) error
}
