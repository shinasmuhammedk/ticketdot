package service

import (
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"ticketdot/internal/auth/model"
)

type LoginResponse struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	User         *model.User `json:"user"`
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

	accessToken, err := GenerateAccessToken(user)
	if err != nil {
		return nil, err
	}

	rawRefreshToken, refreshHash, err := GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	familyID, err := GenerateUUID()
	if err != nil {
		return nil, err
	}

	refreshTokenModel := &model.RefreshToken{
		UserID:    user.ID,
		FamilyID:  familyID,
		TokenHash: refreshHash,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		RevokedAt: nil,
	}

	if err := s.repo.CreateRefreshToken(refreshTokenModel); err != nil {
		return nil, err
	}

	user.PasswordHash = ""

	return &LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: rawRefreshToken,
		User:         user,
	}, nil
}

func (s *AuthService) Refresh(req RefreshRequest) (*RefreshResponse, error) {
	rawToken := strings.TrimSpace(req.RefreshToken)
	if rawToken == "" {
		return nil, errors.New("refresh token is required")
	}

	tokenHash := HashToken(rawToken)

	// Atomically rotate: single UPDATE ... WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > NOW() RETURNING
	rotatedToken, err := s.repo.RotateRefreshToken(tokenHash)
	if err != nil {
		// If rotation did not update any row, inspect if this token previously existed
		existing, getErr := s.repo.GetRefreshToken(tokenHash)
		if getErr == nil && existing != nil && existing.RevokedAt != nil {
			// TOKEN REUSE DETECTED: Immediately revoke all tokens in this family!
			_ = s.repo.RevokeFamily(existing.FamilyID)
			return nil, errors.New("invalid or expired refresh token")
		}
		return nil, errors.New("invalid or expired refresh token")
	}

	user, err := s.repo.GetUserByID(rotatedToken.UserID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	newAccessToken, err := GenerateAccessToken(user)
	if err != nil {
		return nil, err
	}

	newRawToken, newHash, err := GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	// New token maintains the exact same family_id
	newRefreshTokenModel := &model.RefreshToken{
		UserID:    user.ID,
		FamilyID:  rotatedToken.FamilyID,
		TokenHash: newHash,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		RevokedAt: nil,
	}

	if err := s.repo.CreateRefreshToken(newRefreshTokenModel); err != nil {
		return nil, err
	}

	return &RefreshResponse{
		AccessToken:  newAccessToken,
		RefreshToken: newRawToken,
	}, nil
}

func (s *AuthService) Logout(req LogoutRequest) error {
	rawToken := strings.TrimSpace(req.RefreshToken)
	if rawToken != "" {
		tokenHash := HashToken(rawToken)
		_ = s.repo.RevokeRefreshToken(tokenHash)
	}
	return nil
}
