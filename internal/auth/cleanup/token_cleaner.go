package cleanup

import (
	"context"
	"log"
	"time"

	"ticketdot/internal/auth/repository"
)

// StartRefreshTokenCleaner starts a background worker that deletes refresh tokens expired more than 7 days ago.
// Uses PostgreSQL transaction-level advisory locks to ensure safe execution across multiple instances.
func StartRefreshTokenCleaner(ctx context.Context, repo repository.AuthRepository, interval time.Duration) {
	if interval <= 0 {
		interval = 1 * time.Hour
	}

	// Startup pass
	if count, err := repo.DeleteExpiredRefreshTokens(ctx); err == nil && count > 0 {
		log.Printf("[RefreshTokenCleaner] Startup pass: purged %d expired refresh tokens", count)
	}

	ticker := time.NewTicker(interval)

	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Println("[RefreshTokenCleaner] worker stopped gracefully")
				return

			case <-ticker.C:
				count, err := repo.DeleteExpiredRefreshTokens(ctx)
				if err != nil {
					log.Printf("[RefreshTokenCleaner] Error cleaning expired refresh tokens: %v", err)
				} else if count > 0 {
					log.Printf("[RefreshTokenCleaner] Purged %d expired refresh tokens", count)
				}
			}
		}
	}()
}
