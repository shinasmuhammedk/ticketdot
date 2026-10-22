package middleware_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"ticketdot/internal/http/middleware"
)

func TestLoginRateLimit_KeyedByIPAndEmail(t *testing.T) {
	// loginLimit = 3, signupLimit = 10, forgotLimit = 5, globalLimit = 120
	rl := middleware.NewRateLimiter(3, 10, 5, 120, false)

	handler := rl.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	// 1. Send 3 login requests for alice@example.com from IP 1.1.1.1 -> all succeed
	for i := 1; i <= 3; i++ {
		body := `{"email":"Alice@example.com","password":"pass"}`
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(body))
		req.RemoteAddr = "1.1.1.1:12345"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200 OK, got %d", i, rec.Code)
		}
	}

	// 2. 4th request for alice@example.com from same IP must be 429
	body := `{"email":"alice@example.com","password":"pass"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(body))
	req.RemoteAddr = "1.1.1.1:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on 4th login attempt for alice, got %d", rec.Code)
	}

	// 3. Different user bob@example.com from the same IP must still succeed
	bobBody := `{"email":"bob@example.com","password":"pass"}`
	bobReq := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(bobBody))
	bobReq.RemoteAddr = "1.1.1.1:12345"
	bobRec := httptest.NewRecorder()
	handler.ServeHTTP(bobRec, bobReq)

	if bobRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for bob on same IP, got %d", bobRec.Code)
	}

	// 4. Same user alice@example.com from a DIFFERENT IP must succeed
	diffIPReq := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(body))
	diffIPReq.RemoteAddr = "2.2.2.2:12345"
	diffIPRec := httptest.NewRecorder()
	handler.ServeHTTP(diffIPRec, diffIPReq)

	if diffIPRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for alice on different IP, got %d", diffIPRec.Code)
	}
}

func TestSignupRateLimit_KeyedByIP(t *testing.T) {
	// loginLimit = 10, signupLimit = 2, forgotLimit = 5, globalLimit = 120
	rl := middleware.NewRateLimiter(10, 2, 5, 120, false)

	handler := rl.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	for i := 1; i <= 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/auth/signup", bytes.NewBufferString(`{"email":"user@test.com"}`))
		req.RemoteAddr = "3.3.3.3:12345"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("signup %d: expected 200, got %d", i, rec.Code)
		}
	}

	// 3rd request from same IP exceeds signup limit
	req := httptest.NewRequest(http.MethodPost, "/auth/signup", bytes.NewBufferString(`{"email":"another@test.com"}`))
	req.RemoteAddr = "3.3.3.3:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for signup on 3rd attempt, got %d", rec.Code)
	}
}

func TestForgotPasswordRateLimit_KeyedByIPAndEmail(t *testing.T) {
	// forgotLimit = 2
	rl := middleware.NewRateLimiter(10, 10, 2, 120, false)

	handler := rl.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	for i := 1; i <= 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/auth/forgot-password", bytes.NewBufferString(`{"email":"forgot@test.com"}`))
		req.RemoteAddr = "4.4.4.4:12345"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("forgot %d: expected 200, got %d", i, rec.Code)
		}
	}

	// 3rd attempt for same email + IP fails with 429
	req := httptest.NewRequest(http.MethodPost, "/auth/forgot-password", bytes.NewBufferString(`{"email":"forgot@test.com"}`))
	req.RemoteAddr = "4.4.4.4:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for forgot-password, got %d", rec.Code)
	}
}

func TestRequestSizeLimiterReturns413ForOversizedBody(t *testing.T) {
	maxBytes := int64(1024) // 1 KB limit
	limiter := middleware.RequestSizeLimiter(maxBytes)

	handler := limiter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	// Payload larger than 1 KB
	largePayload := bytes.Repeat([]byte("A"), 2048)
	req := httptest.NewRequest(http.MethodPost, "/upload", bytes.NewReader(largePayload))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 Payload Too Large, got %d", rec.Code)
	}
}

func TestSecurityHeadersMiddleware(t *testing.T) {
	handler := middleware.SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("expected X-Content-Type-Options: nosniff, got %q", rec.Header().Get("X-Content-Type-Options"))
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Errorf("expected X-Frame-Options: DENY, got %q", rec.Header().Get("X-Frame-Options"))
	}
	if rec.Header().Get("Referrer-Policy") != "strict-origin-when-cross-origin" {
		t.Errorf("expected Referrer-Policy: strict-origin-when-cross-origin, got %q", rec.Header().Get("Referrer-Policy"))
	}
}
