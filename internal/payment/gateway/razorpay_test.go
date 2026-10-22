package gateway_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ticketdot/internal/payment/gateway"
)

func TestRazorpayCreateOrderSuccess(t *testing.T) {
	testKeyID := "rzp_test_testKey123"
	testKeySecret := "secretKeySecret456"

	var capturedPath string
	var capturedAuth string
	var capturedBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedAuth = r.Header.Get("Authorization")

		_ = json.NewDecoder(r.Body).Decode(&capturedBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":          "order_test_99999",
			"entity":      "order",
			"amount":      99800,
			"amount_paid": 0,
			"amount_due":  99800,
			"currency":    "INR",
			"receipt":     "rcpt_ticket_12345678",
			"status":      "created",
			"created_at":  time.Now().Unix(),
		})
	}))
	defer server.Close()

	rzp := gateway.NewRazorpayGateway(testKeyID, testKeySecret, gateway.WithBaseURL(server.URL))

	in := gateway.CreateOrderInput{
		AmountPaise: 99800,
		Currency:    "INR",
		Receipt:     "rcpt_ticket_12345678",
		Notes: map[string]string{
			"schedule_id": "sched_123",
			"user_id":     "user_456",
			"seats":       "1A,1B",
		},
	}

	order, err := rzp.CreateOrder(context.Background(), in)
	if err != nil {
		t.Fatalf("unexpected error creating order: %v", err)
	}

	// 1. Assert request path
	if capturedPath != "/orders" {
		t.Errorf("expected path /orders, got %q", capturedPath)
	}

	// 2. Assert basic auth header presence without logging it
	if !strings.HasPrefix(capturedAuth, "Basic ") {
		t.Fatalf("expected Basic auth header, got %q", capturedAuth)
	}
	encoded := strings.TrimPrefix(capturedAuth, "Basic ")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("failed to decode basic auth header: %v", err)
	}
	expectedCreds := testKeyID + ":" + testKeySecret
	if string(decoded) != expectedCreds {
		t.Errorf("basic auth credentials mismatch")
	}

	// 3. Assert body fields in paise
	if capturedBody["amount"] == nil || int64(capturedBody["amount"].(float64)) != 99800 {
		t.Errorf("expected amount 99800 paise, got %v", capturedBody["amount"])
	}
	if capturedBody["currency"] != "INR" {
		t.Errorf("expected currency INR, got %v", capturedBody["currency"])
	}
	if capturedBody["receipt"] != "rcpt_ticket_12345678" {
		t.Errorf("expected receipt rcpt_ticket_12345678, got %v", capturedBody["receipt"])
	}

	// 4. Assert returned GatewayOrder
	if order.ID != "order_test_99999" {
		t.Errorf("expected order id order_test_99999, got %q", order.ID)
	}
	if order.AmountPaise != 99800 {
		t.Errorf("expected order amount 99800, got %d", order.AmountPaise)
	}
	if order.Status != "created" {
		t.Errorf("expected order status 'created', got %q", order.Status)
	}
}

func TestRazorpayCreateOrderMinAmountValidation(t *testing.T) {
	rzp := gateway.NewRazorpayGateway("key", "secret")
	in := gateway.CreateOrderInput{
		AmountPaise: 99, // Less than 100 paise
		Currency:    "INR",
		Receipt:     "rcpt_fail",
	}

	_, err := rzp.CreateOrder(context.Background(), in)
	if err == nil {
		t.Fatal("expected error for amount < 100 paise, got nil")
	}
	if !strings.Contains(err.Error(), "100 paise") {
		t.Errorf("expected error mentioning 100 paise, got %v", err)
	}
}

func TestRazorpayCreateOrderErrorMapping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"code":        "BAD_REQUEST_ERROR",
				"description": "Receipt must be at most 40 characters",
				"source":      "business",
				"step":        "order_creation",
				"reason":      "input_validation_failed",
			},
		})
	}))
	defer server.Close()

	rzp := gateway.NewRazorpayGateway("key", "secret", gateway.WithBaseURL(server.URL))
	in := gateway.CreateOrderInput{
		AmountPaise: 50000,
		Currency:    "INR",
		Receipt:     "rcpt_1",
	}

	_, err := rzp.CreateOrder(context.Background(), in)
	if err == nil {
		t.Fatal("expected error from failed gateway call, got nil")
	}

	// Verify generic error returned without exposing internal Razorpay internals to caller
	if !strings.Contains(err.Error(), "payment gateway order creation failed") {
		t.Errorf("expected generic error message, got %v", err)
	}
}

func TestRazorpayCreateOrderTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &http.Client{Timeout: 50 * time.Millisecond}
	rzp := gateway.NewRazorpayGateway("key", "secret", gateway.WithBaseURL(server.URL), gateway.WithHTTPClient(client))

	in := gateway.CreateOrderInput{
		AmountPaise: 50000,
		Currency:    "INR",
		Receipt:     "rcpt_timeout",
	}

	_, err := rzp.CreateOrder(context.Background(), in)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "payment gateway order creation failed") {
		t.Errorf("expected generic bad gateway error on timeout, got %v", err)
	}
}

func TestFakeGateway(t *testing.T) {
	fake := gateway.NewFakeGateway()

	if fake.Name() != "fake" {
		t.Errorf("expected fake gateway name, got %q", fake.Name())
	}

	in := gateway.CreateOrderInput{
		AmountPaise: 49900,
		Currency:    "INR",
		Receipt:     "rcpt_fake_1",
	}

	order, err := fake.CreateOrder(context.Background(), in)
	if err != nil {
		t.Fatalf("unexpected error from fake gateway: %v", err)
	}

	if !strings.HasPrefix(order.ID, "order_fake_") {
		t.Errorf("expected order ID starting with 'order_fake_', got %q", order.ID)
	}
	if order.AmountPaise != 49900 {
		t.Errorf("expected 49900 paise, got %d", order.AmountPaise)
	}
	if order.Status != "created" {
		t.Errorf("expected status 'created', got %q", order.Status)
	}

	// Test minimum amount validation on fake gateway
	_, errMin := fake.CreateOrder(context.Background(), gateway.CreateOrderInput{AmountPaise: 50})
	if errMin == nil {
		t.Fatal("expected error for amount < 100 paise on fake gateway")
	}
}

func TestRazorpayFetchPaymentSuccess(t *testing.T) {
	testKeyID := "rzp_test_key"
	testKeySecret := "secret123"

	var capturedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":          "pay_29QQoUBi66xm2f",
			"entity":      "payment",
			"amount":      50000,
			"currency":    "INR",
			"status":      "captured",
			"order_id":    "order_DBJOWzybf0sJbb",
			"method":      "card",
			"description": "Bus Booking",
			"created_at":  time.Now().Unix(),
		})
	}))
	defer server.Close()

	rzp := gateway.NewRazorpayGateway(testKeyID, testKeySecret, gateway.WithBaseURL(server.URL))

	p, err := rzp.FetchPayment(context.Background(), "pay_29QQoUBi66xm2f")
	if err != nil {
		t.Fatalf("unexpected error fetching payment: %v", err)
	}

	if capturedPath != "/payments/pay_29QQoUBi66xm2f" {
		t.Errorf("expected path /payments/pay_29QQoUBi66xm2f, got %s", capturedPath)
	}
	if p.ID != "pay_29QQoUBi66xm2f" {
		t.Errorf("expected ID pay_29QQoUBi66xm2f, got %s", p.ID)
	}
	if p.OrderID != "order_DBJOWzybf0sJbb" {
		t.Errorf("expected order ID order_DBJOWzybf0sJbb, got %s", p.OrderID)
	}
	if p.AmountPaise != 50000 {
		t.Errorf("expected amount 50000, got %d", p.AmountPaise)
	}
	if p.Status != "captured" {
		t.Errorf("expected status 'captured', got %s", p.Status)
	}
	if p.Method != "card" {
		t.Errorf("expected method 'card', got %s", p.Method)
	}
}

func TestRazorpayFetchPaymentInvalidIDFormat(t *testing.T) {
	// Must reject BEFORE calling network (0 gateway calls)
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	rzp := gateway.NewRazorpayGateway("key", "secret", gateway.WithBaseURL(server.URL))

	invalidIDs := []string{
		"invalid_id",
		"pay_short",       // under 10 chars after pay_
		"pay_with space",  // invalid characters
		"order_1234567890",
		"",
	}

	for _, id := range invalidIDs {
		_, err := rzp.FetchPayment(context.Background(), id)
		if err == nil {
			t.Errorf("expected error for invalid payment ID %q, got nil", id)
		}
	}

	if called {
		t.Fatal("expected 0 gateway calls for invalid payment IDs, but gateway was contacted")
	}
}

func TestRazorpayFetchPaymentNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"code":        "BAD_REQUEST_ERROR",
				"description": "Payment not found",
			},
		})
	}))
	defer server.Close()

	rzp := gateway.NewRazorpayGateway("key", "secret", gateway.WithBaseURL(server.URL))
	_, err := rzp.FetchPayment(context.Background(), "pay_nonexistent123")
	if err == nil {
		t.Fatal("expected error for nonexistent payment, got nil")
	}
}

func TestFakeGatewayFetchPayment(t *testing.T) {
	fake := gateway.NewFakeGateway()

	// 1. Fetching unseeded payment returns not found
	_, err := fake.FetchPayment(context.Background(), "pay_unseeded123")
	if err == nil {
		t.Fatal("expected not found error for unseeded payment, got nil")
	}

	// 2. Fetching with invalid ID format
	_, err = fake.FetchPayment(context.Background(), "bad_id")
	if err == nil {
		t.Fatal("expected error for bad ID format, got nil")
	}

	// 3. Register payment and fetch successfully
	fake.RegisterPayment("pay_registered123", "order_test_456", 80000, "INR", "captured", "upi")
	p, err := fake.FetchPayment(context.Background(), "pay_registered123")
	if err != nil {
		t.Fatalf("unexpected error fetching registered payment: %v", err)
	}
	if p.ID != "pay_registered123" {
		t.Errorf("expected ID pay_registered123, got %s", p.ID)
	}
	if p.OrderID != "order_test_456" {
		t.Errorf("expected order ID order_test_456, got %s", p.OrderID)
	}
	if p.AmountPaise != 80000 {
		t.Errorf("expected amount 80000, got %d", p.AmountPaise)
	}
	if p.Status != "captured" {
		t.Errorf("expected status 'captured', got %s", p.Status)
	}
	if p.Method != "upi" {
		t.Errorf("expected method 'upi', got %s", p.Method)
	}
}

