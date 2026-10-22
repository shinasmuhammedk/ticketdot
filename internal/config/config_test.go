package config_test

import (
	"testing"

	"ticketdot/internal/config"
)

func TestConfigValidationOutsideDevelopment(t *testing.T) {
	tests := []struct {
		name        string
		env         map[string]string
		wantErr     bool
		errContains string
	}{
		{
			name: "missing database url fails",
			env: map[string]string{
				"APP_ENV":                "production",
				"JWT_SECRET":             "super-secret-key-that-is-at-least-32-chars-long",
				"RAZORPAY_KEY_ID":        "rzp_live_test123",
				"RAZORPAY_KEY_SECRET":    "rzp_secret_test123",
				"PAYMENT_WEBHOOK_SECRET": "rzp_webhook_test123",
			},
			wantErr:     true,
			errContains: "DATABASE_URL",
		},
		{
			name: "missing jwt secret fails",
			env: map[string]string{
				"APP_ENV":                "production",
				"DATABASE_URL":           "postgres://localhost:5432/test",
				"RAZORPAY_KEY_ID":        "rzp_live_test123",
				"RAZORPAY_KEY_SECRET":    "rzp_secret_test123",
				"PAYMENT_WEBHOOK_SECRET": "rzp_webhook_test123",
			},
			wantErr:     true,
			errContains: "JWT_SECRET",
		},
		{
			name: "short jwt secret fails outside development",
			env: map[string]string{
				"APP_ENV":                "production",
				"DATABASE_URL":           "postgres://localhost:5432/test",
				"JWT_SECRET":             "short-secret-under-32",
				"RAZORPAY_KEY_ID":        "rzp_live_test123",
				"RAZORPAY_KEY_SECRET":    "rzp_secret_test123",
				"PAYMENT_WEBHOOK_SECRET": "rzp_webhook_test123",
			},
			wantErr:     true,
			errContains: "at least 32 characters",
		},
		{
			name: "missing razorpay key id fails outside development",
			env: map[string]string{
				"APP_ENV":                "production",
				"DATABASE_URL":           "postgres://localhost:5432/test",
				"JWT_SECRET":             "super-secret-key-that-is-at-least-32-chars-long",
				"RAZORPAY_KEY_SECRET":    "rzp_secret_test123",
				"PAYMENT_WEBHOOK_SECRET": "rzp_webhook_test123",
			},
			wantErr:     true,
			errContains: "RAZORPAY_KEY_ID",
		},
		{
			name: "missing razorpay secret fails outside development",
			env: map[string]string{
				"APP_ENV":                "production",
				"DATABASE_URL":           "postgres://localhost:5432/test",
				"JWT_SECRET":             "super-secret-key-that-is-at-least-32-chars-long",
				"RAZORPAY_KEY_ID":        "rzp_live_test123",
				"PAYMENT_WEBHOOK_SECRET": "rzp_webhook_test123",
			},
			wantErr:     true,
			errContains: "RAZORPAY_KEY_SECRET",
		},
		{
			name: "missing webhook secret fails outside development",
			env: map[string]string{
				"APP_ENV":             "production",
				"DATABASE_URL":        "postgres://localhost:5432/test",
				"JWT_SECRET":          "super-secret-key-that-is-at-least-32-chars-long",
				"RAZORPAY_KEY_ID":     "rzp_live_test123",
				"RAZORPAY_KEY_SECRET": "rzp_secret_test123",
			},
			wantErr:     true,
			errContains: "PAYMENT_WEBHOOK_SECRET",
		},
		{
			name: "unknown APP_ENV is treated as production and enforces secrets",
			env: map[string]string{
				"APP_ENV":      "unknown_custom_env",
				"DATABASE_URL": "postgres://localhost:5432/test",
				"JWT_SECRET":   "dev-secret", // short secret should fail
			},
			wantErr:     true,
			errContains: "at least 32 characters",
		},
		{
			name: "unset APP_ENV is treated as production and enforces secrets",
			env: map[string]string{
				"APP_ENV":      "",
				"DATABASE_URL": "postgres://localhost:5432/test",
				"JWT_SECRET":   "dev-secret",
			},
			wantErr:     true,
			errContains: "at least 32 characters",
		},
		{
			name: "fake payment gateway rejected outside development",
			env: map[string]string{
				"APP_ENV":                "production",
				"DATABASE_URL":           "postgres://localhost:5432/test",
				"JWT_SECRET":             "super-secret-key-that-is-at-least-32-chars-long",
				"RAZORPAY_KEY_ID":        "rzp_live_test123",
				"RAZORPAY_KEY_SECRET":    "rzp_secret_test123",
				"PAYMENT_WEBHOOK_SECRET": "rzp_webhook_test123",
				"PAYMENT_GATEWAY":        "fake",
			},
			wantErr:     true,
			errContains: "fake is only allowed in development",
		},
		{
			name: "invalid payment gateway rejected",
			env: map[string]string{
				"APP_ENV":                "production",
				"DATABASE_URL":           "postgres://localhost:5432/test",
				"JWT_SECRET":             "super-secret-key-that-is-at-least-32-chars-long",
				"RAZORPAY_KEY_ID":        "rzp_live_test123",
				"RAZORPAY_KEY_SECRET":    "rzp_secret_test123",
				"PAYMENT_WEBHOOK_SECRET": "rzp_webhook_test123",
				"PAYMENT_GATEWAY":        "stripe",
			},
			wantErr:     true,
			errContains: "must be 'razorpay' or 'fake'",
		},
		{
			name: "valid production config passes",
			env: map[string]string{
				"APP_ENV":                "production",
				"DATABASE_URL":           "postgres://localhost:5432/test",
				"JWT_SECRET":             "super-secret-key-that-is-at-least-32-chars-long",
				"RAZORPAY_KEY_ID":        "rzp_live_test123",
				"RAZORPAY_KEY_SECRET":    "rzp_secret_test123",
				"PAYMENT_WEBHOOK_SECRET": "rzp_webhook_test123",
				"ALLOWED_ORIGINS":        "https://ticketdot.com, https://admin.ticketdot.com",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup := func(key string) string {
				return tt.env[key]
			}
			cfg, err := config.LoadFromLookup(lookup)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Fatalf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if cfg == nil {
					t.Fatal("expected config, got nil")
				}
				if len(cfg.AllowedOrigins) != 2 {
					t.Fatalf("expected 2 allowed origins, got %d", len(cfg.AllowedOrigins))
				}
			}
		})
	}
}

func TestConfigDevelopmentFallback(t *testing.T) {
	env := map[string]string{
		"APP_ENV":      "development",
		"DATABASE_URL": "postgres://localhost:5432/test",
		"JWT_SECRET":   "dev-secret", // short secret allowed in dev
	}
	lookup := func(key string) string {
		return env[key]
	}

	cfg, err := config.LoadFromLookup(lookup)
	if err != nil {
		t.Fatalf("development config should fall back and not fail: %v", err)
	}

	if !cfg.IsDevelopment() {
		t.Fatal("expected IsDevelopment() to be true")
	}

	if cfg.PaymentGateway != "razorpay" {
		t.Fatalf("expected default PAYMENT_GATEWAY to be 'razorpay', got %q", cfg.PaymentGateway)
	}

	if cfg.PaymentWindowMinutes != 12 {
		t.Fatalf("expected default PAYMENT_WINDOW_MINUTES to be 12, got %d", cfg.PaymentWindowMinutes)
	}

	// Verify fake gateway is accepted in development
	env["PAYMENT_GATEWAY"] = "fake"
	cfgFake, err := config.LoadFromLookup(lookup)
	if err != nil {
		t.Fatalf("expected PAYMENT_GATEWAY=fake to be allowed in development: %v", err)
	}
	if cfgFake.PaymentGateway != "fake" {
		t.Fatalf("expected PaymentGateway to be 'fake', got %q", cfgFake.PaymentGateway)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
