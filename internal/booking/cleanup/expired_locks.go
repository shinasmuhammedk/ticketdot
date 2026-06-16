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