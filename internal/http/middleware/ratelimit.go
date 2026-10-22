package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	authmiddleware "ticketdot/internal/auth/middleware"
	"ticketdot/internal/httpx"
)

type tokenBucket struct {
	tokens     float64
	lastRefill time.Time
}

type RateLimiter struct {
	mu                      sync.Mutex
	loginRateLimit          int  // requests per 15 minutes per IP + email (default: 10)
	signupRateLimit         int  // requests per hour per IP (default: 10)
	forgotPasswordRateLimit int  // requests per hour per IP + email (default: 5)
	globalRateLimit         int  // requests per minute per IP (default: 120)
	webhookRateLimit        int  // requests per minute per IP for webhooks (default: 300)
	trustProxy              bool // whether to trust X-Forwarded-For
	buckets                 map[string]*tokenBucket
}

func (rl *RateLimiter) SetWebhookRateLimit(limit int) *RateLimiter {
	if limit > 0 {
		rl.webhookRateLimit = limit
	}
	return rl
}

func NewRateLimiter(loginLimit, signupLimit, forgotPasswordLimit, globalLimit int, trustProxy bool) *RateLimiter {
	if loginLimit <= 0 {
		loginLimit = 10
	}
	if signupLimit <= 0 {
		signupLimit = 10
	}
	if forgotPasswordLimit <= 0 {
		forgotPasswordLimit = 5
	}
	if globalLimit <= 0 {
		globalLimit = 120
	}

	rl := &RateLimiter{
		loginRateLimit:          loginLimit,
		signupRateLimit:         signupLimit,
		forgotPasswordRateLimit: forgotPasswordLimit,
		globalRateLimit:         globalLimit,
		webhookRateLimit:        300,
		trustProxy:              trustProxy,
		buckets:                 make(map[string]*tokenBucket),
	}

	// Periodic cleanup of idle buckets every 10 minutes
	go rl.cleanupRoutine(10*time.Minute, 2*time.Hour)

	return rl
}

func (rl *RateLimiter) cleanupRoutine(interval, maxIdle time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for key, b := range rl.buckets {
			if now.Sub(b.lastRefill) > maxIdle {
				delete(rl.buckets, key)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) getClientIP(r *http.Request) string {
	if rl.trustProxy {
		xff := r.Header.Get("X-Forwarded-For")
		if xff != "" {
			parts := strings.Split(xff, ",")
			clientIP := strings.TrimSpace(parts[0])
			if clientIP != "" {
				return clientIP
			}
		}
		xri := strings.TrimSpace(r.Header.Get("X-Real-IP"))
		if xri != "" {
			return xri
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

func extractEmailFromBody(r *http.Request) string {
	if r.Body == nil {
		return ""
	}
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return ""
	}
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	var payload struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(bodyBytes, &payload); err == nil {
		return strings.ToLower(strings.TrimSpace(payload.Email))
	}
	return ""
}

// Handler returns a standard middleware handler for rate limiting.
func (rl *RateLimiter) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := rl.getClientIP(r)
		p := strings.TrimRight(r.URL.Path, "/")

		var key string
		var capacity float64
		var windowSeconds float64

		switch p {
		case "/auth/login":
			email := extractEmailFromBody(r)
			key = "login:" + ip + ":" + email
			capacity = float64(rl.loginRateLimit)
			windowSeconds = 15.0 * 60.0 // 15 minutes
		case "/auth/signup":
			key = "signup:" + ip
			capacity = float64(rl.signupRateLimit)
			windowSeconds = 60.0 * 60.0 // 1 hour
		case "/auth/forgot-password":
			email := extractEmailFromBody(r)
			key = "forgot:" + ip + ":" + email
			capacity = float64(rl.forgotPasswordRateLimit)
			windowSeconds = 60.0 * 60.0 // 1 hour
		case "/payments/webhook":
			key = "webhook:" + ip
			capacity = float64(rl.webhookRateLimit)
			windowSeconds = 60.0 // 1 minute
		default:
			key = "global:" + ip
			capacity = float64(rl.globalRateLimit)
			windowSeconds = 60.0 // 1 minute
		}

		refillRate := capacity / windowSeconds // tokens per second
		now := time.Now()

		rl.mu.Lock()
		b, exists := rl.buckets[key]
		if !exists {
			b = &tokenBucket{
				tokens:     capacity - 1.0,
				lastRefill: now,
			}
			rl.buckets[key] = b
			rl.mu.Unlock()
			next.ServeHTTP(w, r)
			return
		}

		elapsed := now.Sub(b.lastRefill).Seconds()
		b.tokens += elapsed * refillRate
		if b.tokens > capacity {
			b.tokens = capacity
		}
		b.lastRefill = now

		if b.tokens >= 1.0 {
			b.tokens -= 1.0
			rl.mu.Unlock()
			next.ServeHTTP(w, r)
			return
		}

		// Rate limit exceeded
		needed := 1.0 - b.tokens
		retryAfter := int(math.Ceil(needed / refillRate))
		if retryAfter < 1 {
			retryAfter = 1
		}
		rl.mu.Unlock()

		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
		httpx.WriteError(w, r, httpx.RateLimit("rate limit exceeded, please retry later"))
	})
}

// UserRateLimiter enforces token-bucket rate limits keyed by authenticated user ID (or client IP fallback).
type UserRateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	buckets map[string]*tokenBucket
}

func NewUserRateLimiter(limit int, window time.Duration) func(http.Handler) http.Handler {
	if limit <= 0 {
		limit = 10
	}
	if window <= 0 {
		window = time.Minute
	}

	url := &UserRateLimiter{
		limit:   limit,
		window:  window,
		buckets: make(map[string]*tokenBucket),
	}

	// Periodic cleanup of stale user buckets
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C {
			url.mu.Lock()
			now := time.Now()
			for k, b := range url.buckets {
				if now.Sub(b.lastRefill) > 15*time.Minute {
					delete(url.buckets, k)
				}
			}
			url.mu.Unlock()
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, _ := r.Context().Value(authmiddleware.UserIDKey).(string)
			key := strings.TrimSpace(userID)
			if key == "" {
				host, _, err := net.SplitHostPort(r.RemoteAddr)
				if err == nil && host != "" {
					key = host
				} else {
					key = r.RemoteAddr
				}
			}

			capacity := float64(url.limit)
			windowSec := url.window.Seconds()
			refillRate := capacity / windowSec
			now := time.Now()

			url.mu.Lock()
			b, exists := url.buckets[key]
			if !exists {
				url.buckets[key] = &tokenBucket{
					tokens:     capacity - 1.0,
					lastRefill: now,
				}
				url.mu.Unlock()
				next.ServeHTTP(w, r)
				return
			}

			elapsed := now.Sub(b.lastRefill).Seconds()
			b.tokens += elapsed * refillRate
			if b.tokens > capacity {
				b.tokens = capacity
			}
			b.lastRefill = now

			if b.tokens >= 1.0 {
				b.tokens -= 1.0
				url.mu.Unlock()
				next.ServeHTTP(w, r)
				return
			}

			needed := 1.0 - b.tokens
			retryAfter := int(math.Ceil(needed / refillRate))
			if retryAfter < 1 {
				retryAfter = 1
			}
			url.mu.Unlock()

			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			httpx.WriteError(w, r, httpx.RateLimit("payment verification rate limit exceeded, please retry later"))
		})
	}
}
