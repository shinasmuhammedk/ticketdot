package service

import (
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"ticketdot/internal/auth/model"
)

func TestTokenGenerationAndValidation(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-12345")

	user := &model.User{
		ID:    "user-123",
		Email: "test@example.com",
		Role:  "passenger",
	}

	tokenStr, err := GenerateAccessToken(user)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	claims, err := ValidateAccessToken(tokenStr)
	if err != nil {
		t.Fatalf("ValidateAccessToken failed: %v", err)
	}

	if claims["user_id"] != user.ID {
		t.Errorf("expected user_id %s, got %v", user.ID, claims["user_id"])
	}

	expFloat, ok := claims["exp"].(float64)
	if !ok {
		t.Fatalf("exp claim missing or not float64")
	}

	// Verify expiration is ~15 minutes from now (allow 1-minute tolerance)
	expectedExp := time.Now().Add(15 * time.Minute).Unix()
	if int64(expFloat) < expectedExp-60 || int64(expFloat) > expectedExp+60 {
		t.Errorf("expected exp around %d (15m), got %d", expectedExp, int64(expFloat))
	}
}

func TestValidateAccessToken_RejectNonHMAC(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-12345")

	// Token with alg: "none"
	claims := jwt.MapClaims{
		"user_id": "attacker",
		"email":   "attacker@evil.com",
		"role":    "super_admin",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tokenString, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("failed to create none token: %v", err)
	}

	_, err = ValidateAccessToken(tokenString)
	if err == nil {
		t.Fatalf("expected ValidateAccessToken to reject alg:none token, but succeeded")
	}
}
