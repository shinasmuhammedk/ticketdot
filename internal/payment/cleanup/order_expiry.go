package cleanup

import (
	"context"
	"log"
	"time"

	"ticketdot/internal/payment/repository"
)

// StartOrderExpiryWorker starts a background worker that transitions stale pending payment orders
// whose expires_at has passed into the 'expired' status.
// Uses PostgreSQL transaction-level advisory locks to ensure safe execution across multiple instances.
func StartOrderExpiryWorker(ctx context.Context, repo repository.PaymentRepository, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}

	// Startup pass
	if count, err := repo.ExpireStalePendingOrders(ctx); err == nil && count > 0 {
		log.Printf("[OrderExpiryWorker] Startup pass: expired %d stale payment orders", count)
	}

	ticker := time.NewTicker(interval)

	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Println("[OrderExpiryWorker] worker stopped gracefully")
				return

			case <-ticker.C:
				count, err := repo.ExpireStalePendingOrders(ctx)
				if err != nil {
					log.Printf("[OrderExpiryWorker] Error expiring stale payment orders: %v", err)
				} else if count > 0 {
					log.Printf("[OrderExpiryWorker] Expired %d stale payment orders", count)
				}
			}
		}
	}()
}
