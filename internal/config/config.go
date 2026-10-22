package config

import (
	"errors"
	"log"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	DatabaseURL          string
	JWTSecret            string
	Port                 string
	AppEnv               string
	RazorpayKeyID        string
	RazorpayKeySecret    string
	PaymentWebhookSecret    string
	PaymentGateway          string
	PaymentWindowMinutes    int
	MaxOrderAmountPaise     int64
	AllowedOrigins          []string
	AuthRateLimit           int
	LoginRateLimit          int
	SignupRateLimit         int
	ForgotPasswordRateLimit int
	GlobalRateLimit         int
	PaymentVerifyRateLimit  int
	WebhookRateLimit        int
	TrustProxy              bool
	MaxBodyBytes            int64
}

func (c *Config) IsDevelopment() bool {
	return strings.EqualFold(strings.TrimSpace(c.AppEnv), "development")
}

// Load loads and validates configuration from environment variables.
func Load() (*Config, error) {
	return LoadFromLookup(os.Getenv)
}

// LoadFromLookup loads and validates configuration using a lookup function for environment variables.
func LoadFromLookup(lookup func(string) string) (*Config, error) {
	databaseURL := strings.TrimSpace(lookup("DATABASE_URL"))
	if databaseURL == "" {
		return nil, errors.New("missing required environment variable: DATABASE_URL")
	}

	jwtSecret := strings.TrimSpace(lookup("JWT_SECRET"))
	if jwtSecret == "" {
		return nil, errors.New("missing required environment variable: JWT_SECRET")
	}

	port := strings.TrimSpace(lookup("PORT"))
	if port == "" {
		port = "8080"
	}

	appEnv := strings.TrimSpace(lookup("APP_ENV"))
	if appEnv == "" {
		log.Println("WARNING: APP_ENV is unset; defaulting to 'production'")
		appEnv = "production"
	}
	isDev := strings.EqualFold(appEnv, "development")

	if !isDev && len(jwtSecret) < 32 {
		return nil, errors.New("JWT_SECRET must be at least 32 characters in non-development environments")
	}

	keyID := strings.TrimSpace(lookup("RAZORPAY_KEY_ID"))
	keySecret := strings.TrimSpace(lookup("RAZORPAY_KEY_SECRET"))
	webhookSecret := strings.TrimSpace(lookup("PAYMENT_WEBHOOK_SECRET"))

	if isDev {
		if keyID == "" {
			keyID = "rzp_test_dev_dummy_key"
			log.Println("WARNING: RAZORPAY_KEY_ID is not set; using development fallback placeholder")
		}
		if keySecret == "" {
			keySecret = "rzp_test_dev_dummy_secret"
			log.Println("WARNING: RAZORPAY_KEY_SECRET is not set; using development fallback placeholder")
		}
		if webhookSecret == "" {
			webhookSecret = "rzp_test_dev_dummy_webhook_secret"
			log.Println("WARNING: PAYMENT_WEBHOOK_SECRET is not set; using development fallback placeholder")
		}
	} else {
		if keyID == "" {
			return nil, errors.New("missing required environment variable: RAZORPAY_KEY_ID")
		}
		if keySecret == "" {
			return nil, errors.New("missing required environment variable: RAZORPAY_KEY_SECRET")
		}
		if webhookSecret == "" {
			return nil, errors.New("missing required environment variable: PAYMENT_WEBHOOK_SECRET")
		}
	}

	paymentGateway := strings.ToLower(strings.TrimSpace(lookup("PAYMENT_GATEWAY")))
	if paymentGateway == "" {
		paymentGateway = "razorpay"
	}
	if paymentGateway != "razorpay" && paymentGateway != "fake" {
		return nil, errors.New("invalid PAYMENT_GATEWAY: must be 'razorpay' or 'fake'")
	}
	if paymentGateway == "fake" && !isDev {
		return nil, errors.New("PAYMENT_GATEWAY=fake is only allowed in development environment")
	}

	paymentWindowMinutes := 12
	if rawWindow := strings.TrimSpace(lookup("PAYMENT_WINDOW_MINUTES")); rawWindow != "" {
		if n, err := strconv.Atoi(rawWindow); err == nil && n > 0 {
			paymentWindowMinutes = n
		}
	}

	maxOrderAmountPaise := int64(10000000) // Default 100,000 Rupees in paise (10,000,000 paise)
	if rawMaxOrder := strings.TrimSpace(lookup("MAX_ORDER_AMOUNT_PAISE")); rawMaxOrder != "" {
		if n, err := strconv.ParseInt(rawMaxOrder, 10, 64); err == nil && n > 0 {
			maxOrderAmountPaise = n
		}
	}

	var allowedOrigins []string
	rawOrigins := strings.TrimSpace(lookup("ALLOWED_ORIGINS"))
	if rawOrigins != "" {
		parts := strings.Split(rawOrigins, ",")
		for _, p := range parts {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				allowedOrigins = append(allowedOrigins, trimmed)
			}
		}
	}

	if len(allowedOrigins) == 0 && isDev {
		allowedOrigins = []string{"http://localhost:5173", "http://localhost:5174"}
	}

	authRateLimit := 5
	if rawAuthLimit := strings.TrimSpace(lookup("AUTH_RATE_LIMIT")); rawAuthLimit != "" {
		if n, err := strconv.Atoi(rawAuthLimit); err == nil && n > 0 {
			authRateLimit = n
		}
	}

	loginRateLimit := 10
	if rawLoginLimit := strings.TrimSpace(lookup("LOGIN_RATE_LIMIT")); rawLoginLimit != "" {
		if n, err := strconv.Atoi(rawLoginLimit); err == nil && n > 0 {
			loginRateLimit = n
		}
	}

	signupRateLimit := 10
	if rawSignupLimit := strings.TrimSpace(lookup("SIGNUP_RATE_LIMIT")); rawSignupLimit != "" {
		if n, err := strconv.Atoi(rawSignupLimit); err == nil && n > 0 {
			signupRateLimit = n
		}
	}

	forgotPasswordRateLimit := 5
	if rawForgotLimit := strings.TrimSpace(lookup("FORGOT_PASSWORD_RATE_LIMIT")); rawForgotLimit != "" {
		if n, err := strconv.Atoi(rawForgotLimit); err == nil && n > 0 {
			forgotPasswordRateLimit = n
		}
	}

	globalRateLimit := 120
	if rawGlobalLimit := strings.TrimSpace(lookup("GLOBAL_RATE_LIMIT")); rawGlobalLimit != "" {
		if n, err := strconv.Atoi(rawGlobalLimit); err == nil && n > 0 {
			globalRateLimit = n
		}
	}

	trustProxy := false
	if rawTrustProxy := strings.TrimSpace(lookup("TRUST_PROXY")); rawTrustProxy != "" {
		if b, err := strconv.ParseBool(rawTrustProxy); err == nil {
			trustProxy = b
		}
	}

	maxBodyBytes := int64(1048576) // 1 MB default
	if rawMaxBody := strings.TrimSpace(lookup("MAX_BODY_BYTES")); rawMaxBody != "" {
		if n, err := strconv.ParseInt(rawMaxBody, 10, 64); err == nil && n > 0 {
			maxBodyBytes = n
		}
	}

	paymentVerifyRateLimit := 10
	if rawVerifyLimit := strings.TrimSpace(lookup("RATE_LIMIT_PAYMENT_VERIFY_PER_MIN")); rawVerifyLimit != "" {
		if n, err := strconv.Atoi(rawVerifyLimit); err == nil && n > 0 {
			paymentVerifyRateLimit = n
		}
	} else if rawVerifyLimit := strings.TrimSpace(lookup("PAYMENT_VERIFY_RATE_LIMIT")); rawVerifyLimit != "" {
		if n, err := strconv.Atoi(rawVerifyLimit); err == nil && n > 0 {
			paymentVerifyRateLimit = n
		}
	}

	webhookRateLimit := 300
	if rawWebhookLimit := strings.TrimSpace(lookup("RATE_LIMIT_WEBHOOK_PER_MIN")); rawWebhookLimit != "" {
		if n, err := strconv.Atoi(rawWebhookLimit); err == nil && n > 0 {
			webhookRateLimit = n
		}
	} else if rawWebhookLimit := strings.TrimSpace(lookup("WEBHOOK_RATE_LIMIT")); rawWebhookLimit != "" {
		if n, err := strconv.Atoi(rawWebhookLimit); err == nil && n > 0 {
			webhookRateLimit = n
		}
	}

	return &Config{
		DatabaseURL:             databaseURL,
		JWTSecret:               jwtSecret,
		Port:                    port,
		AppEnv:                  appEnv,
		RazorpayKeyID:           keyID,
		RazorpayKeySecret:       keySecret,
		PaymentWebhookSecret:    webhookSecret,
		PaymentGateway:          paymentGateway,
		PaymentWindowMinutes:    paymentWindowMinutes,
		MaxOrderAmountPaise:     maxOrderAmountPaise,
		AllowedOrigins:          allowedOrigins,
		AuthRateLimit:           authRateLimit,
		LoginRateLimit:          loginRateLimit,
		SignupRateLimit:         signupRateLimit,
		ForgotPasswordRateLimit: forgotPasswordRateLimit,
		GlobalRateLimit:         globalRateLimit,
		PaymentVerifyRateLimit:  paymentVerifyRateLimit,
		WebhookRateLimit:        webhookRateLimit,
		TrustProxy:              trustProxy,
		MaxBodyBytes:            maxBodyBytes,
	}, nil
}
