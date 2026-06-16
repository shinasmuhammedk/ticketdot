package service

import (
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"ticketdot/internal/auth/model"
)

type LoginResponse struct {
	AccessToken string      `json:"access_token"`
	User        *model.User `json:"user"`
}

func (s *AuthService) Login(req LoginRequest) (*LoginResponse, error) {
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	if req.Email == "" {
		return nil, errors.New("email is required")
	}

	if req.Password == "" {
		return nil, errors.New("password is required")
	}

	user, err := s.repo.GetUserByEmail(req.Email)
	if err != nil {
		return nil, errors.New("invalid email or password")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		return nil, errors.New("invalid email or password")
	}

	token, err := GenerateAccessToken(user)
	if err != nil {
		return nil, err
	}

	user.PasswordHash = ""

	return &LoginResponse{
		AccessToken: token,
		User:        user,
	}, nil
}