package cleanup

import (
	"context"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
)

func StartExpiredLockCleaner(ctx context.Context, db *sqlx.DB) {
	ticker := time.NewTicker(1 * time.Minute)

	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Println("expired lock cleaner stopped")
				return

			case <-ticker.C:
				releaseExpiredLocks(db)
			}
		}
	}()
}

func ReleaseExpiredLocks(db *sqlx.DB) {
	releaseExpiredLocks(db)
}

func releaseExpiredLocks(db *sqlx.DB) {
	query := `
		UPDATE seat_inventory
		SET
			status = 'available',
			locked_by = NULL,
			locked_at = NULL,
			lock_expires = NULL
		WHERE status = 'locked'
		  AND lock_expires < NOW()
		  AND NOT EXISTS (
			SELECT 1 FROM payments p
			WHERE p.schedule_id = seat_inventory.schedule_id
			  AND seat_inventory.seat_number = ANY(p.seats)
			  AND p.status = 'pending'
			  AND (p.expires_at IS NULL OR p.expires_at > NOW())
		  )
	`

	result, err := db.Exec(query)
	if err != nil {
		log.Println("failed to release expired locks:", err)
		return
	}

	count, _ := result.RowsAffected()
	if count > 0 {
		log.Println("released expired seat locks:", count)
	}
}
