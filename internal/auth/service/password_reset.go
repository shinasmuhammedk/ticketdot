package service

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"ticketdot/internal/auth/model"
)

func (s *AuthService) ForgotPassword(req ForgotPasswordRequest) (*ForgotPasswordResponse, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		return nil, errors.New("email is required")
	}

	user, err := s.repo.GetUserByEmail(email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Don't leak whether user exists for security
			return &ForgotPasswordResponse{
				Message: "If an account with that email exists, reset instructions have been generated.",
			}, nil
		}
		return nil, err
	}

	// Generate 32-byte crypto-random hex token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, errors.New("failed to generate reset token")
	}
	token := hex.EncodeToString(tokenBytes)

	// Valid for 1 hour
	expiresAt := time.Now().Add(1 * time.Hour)

	reset := &model.PasswordReset{
		UserID:    user.ID,
		Email:     user.Email,
		Token:     token,
		ExpiresAt: expiresAt,
		Used:      false,
	}

	if err := s.repo.CreatePasswordReset(reset); err != nil {
		return nil, err
	}

	return &ForgotPasswordResponse{
		Message:    "Password reset instructions have been generated successfully.",
		ResetToken: token,
	}, nil
}

func (s *AuthService) ResetPassword(req ResetPasswordRequest) error {
	token := strings.TrimSpace(req.Token)
	if token == "" {
		return errors.New("reset token is required")
	}

	if len(req.NewPassword) < 6 {
		return errors.New("password must be at least 6 characters")
	}

	reset, err := s.repo.GetValidPasswordReset(token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("invalid or expired password reset token")
		}
		return err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	if err := s.repo.UpdateUserPassword(reset.UserID, string(hashedPassword)); err != nil {
		return err
	}

	if err := s.repo.MarkPasswordResetUsed(token); err != nil {
		return err
	}

	return nil
}
