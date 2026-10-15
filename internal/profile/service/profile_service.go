package service

import (
	"errors"
	"strings"

	"ticketdot/internal/profile/model"
	"ticketdot/internal/profile/repository"
)

type ProfileService struct {
	repo repository.Repository
}

func NewProfileService(
	repo repository.Repository,
) *ProfileService {
	return &ProfileService{
		repo: repo,
	}
}

func (s *ProfileService) GetProfile(
	userID string,
) (*model.ProfileResponse, error) {

	userID = strings.TrimSpace(userID)

	if userID == "" {
		return nil, errors.New("user id is required")
	}

	return s.repo.GetProfile(userID)
}

func (s *ProfileService) UpdateProfile(
	userID string,
	req *model.UpdateProfileRequest,
) error {

	userID = strings.TrimSpace(userID)

	if userID == "" {
		return errors.New("user id is required")
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Phone = strings.TrimSpace(req.Phone)

	if req.Name == "" {
		return errors.New("name is required")
	}

	if req.Phone == "" {
		return errors.New("phone is required")
	}

	return s.repo.UpdateProfile(
		userID,
		req,
	)
}
