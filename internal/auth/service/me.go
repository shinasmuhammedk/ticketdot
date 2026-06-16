package service

import "ticketdot/internal/auth/model"

func (s *AuthService) Me(userID string) (*model.User, error) {
	user, err := s.repo.GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	user.PasswordHash = ""
	return user, nil
}