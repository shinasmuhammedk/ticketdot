package repository

import (
	"regexp"
	"testing"
)

func TestGenerateBookingRef(t *testing.T) {
	pattern := regexp.MustCompile(`^[A-Z0-9]{8}$`)

	seen := make(map[string]bool)
	count := 500

	for i := 0; i < count; i++ {
		ref, err := generateBookingRef()
		if err != nil {
			t.Fatalf("generateBookingRef failed: %v", err)
		}

		if len(ref) != 8 {
			t.Fatalf("expected 8 characters, got %d (%s)", len(ref), ref)
		}

		if !pattern.MatchString(ref) {
			t.Fatalf("expected alphanumeric uppercase 8-char code, got %s", ref)
		}

		if seen[ref] {
			t.Fatalf("collision detected for random code %s after %d iterations", ref, i)
		}
		seen[ref] = true
	}
}
