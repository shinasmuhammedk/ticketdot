package service_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"ticketdot/internal/payment/service"
)

func TestPaymentHMACSignatureVerification(t *testing.T) {
	secret := "test_secret_12345"
	orderID := "order_ABCD1234EFGH5678"
	paymentID := "pay_XYZ9876543210"

	// Generate signature manually
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(orderID + "|" + paymentID))
	expected := hex.EncodeToString(mac.Sum(nil))

	computed := service.ComputeHmacSha256(orderID+"|"+paymentID, secret)
	if computed != expected {
		t.Fatalf("expected signature %s, got %s", expected, computed)
	}

	// Tampered paymentID should fail
	tampered := service.ComputeHmacSha256(orderID+"|pay_TAMPERED", secret)
	if tampered == expected {
		t.Fatalf("tampered signature should not match expected")
	}
}
