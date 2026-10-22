package middleware

import (
	"net/http"

	"ticketdot/internal/httpx"
)

// RequestSizeLimiter limits the maximum readable size of incoming request bodies.
func RequestSizeLimiter(maxBytes int64) func(http.Handler) http.Handler {
	if maxBytes <= 0 {
		maxBytes = 1048576 // 1 MB default
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > maxBytes {
				httpx.WriteError(w, r, httpx.PayloadTooLarge("request body exceeds maximum allowed size"))
				return
			}
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}
