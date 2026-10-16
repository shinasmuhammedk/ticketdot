package lifecycle

import (
	"context"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
)

type TransitionStats struct {
	DepartedCount      int64
	CompletedCount     int64
	LocksReleasedCount int64
}

// StartScheduleLifecycleWorker starts a background worker that monitors schedule timings
// and automatically transitions statuses ('scheduled' -> 'departed' -> 'completed').
func StartScheduleLifecycleWorker(ctx context.Context, db *sqlx.DB, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}

	// Immediate pass upon startup
	if stats, err := TransitionSchedules(ctx, db); err == nil {
		if stats.DepartedCount > 0 || stats.CompletedCount > 0 || stats.LocksReleasedCount > 0 {
			log.Printf("[ScheduleLifecycle] Startup pass: %d departed, %d completed, %d stale locks cleaned",
				stats.DepartedCount, stats.CompletedCount, stats.LocksReleasedCount)
		}
	} else {
		log.Printf("[ScheduleLifecycle] Startup pass error: %v", err)
	}

	ticker := time.NewTicker(interval)

	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Println("[ScheduleLifecycle] worker stopped gracefully")
				return

			case <-ticker.C:
				stats, err := TransitionSchedules(ctx, db)
				if err != nil {
					log.Printf("[ScheduleLifecycle] Error transitioning schedules: %v", err)
				} else if stats.DepartedCount > 0 || stats.CompletedCount > 0 || stats.LocksReleasedCount > 0 {
					log.Printf("[ScheduleLifecycle] Transitioned: %d departed, %d completed, %d stale locks cleaned",
						stats.DepartedCount, stats.CompletedCount, stats.LocksReleasedCount)
				}
			}
		}
	}()
}

// TransitionSchedules performs atomic batch transitions for schedules past their departure or arrival times.
func TransitionSchedules(ctx context.Context, db *sqlx.DB) (TransitionStats, error) {
	var stats TransitionStats

	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return stats, err
	}
	defer tx.Rollback()

	// 1. Transition 'scheduled' trips to 'departed' once departure_time <= NOW() and arrival_time > NOW()
	departedRes, err := tx.ExecContext(ctx, `
		UPDATE schedules
		SET status = 'departed'
		WHERE status = 'scheduled'
		  AND departure_time <= NOW()
		  AND arrival_time > NOW()
	`)
	if err != nil {
		return stats, err
	}
	stats.DepartedCount, _ = departedRes.RowsAffected()

	// 2. Transition 'scheduled' or 'departed' trips to 'completed' once arrival_time <= NOW()
	completedRes, err := tx.ExecContext(ctx, `
		UPDATE schedules
		SET status = 'completed'
		WHERE status IN ('scheduled', 'departed')
		  AND arrival_time <= NOW()
	`)
	if err != nil {
		return stats, err
	}
	stats.CompletedCount, _ = completedRes.RowsAffected()

	// 3. Clear any dangling seat locks for schedules that are departed or completed
	locksRes, err := tx.ExecContext(ctx, `
		UPDATE seat_inventory
		SET
			status = 'available',
			locked_by = NULL,
			locked_at = NULL,
			lock_expires = NULL
		WHERE status = 'locked'
		  AND schedule_id IN (
			SELECT id FROM schedules WHERE status IN ('departed', 'completed', 'cancelled')
		  )
	`)
	if err != nil {
		return stats, err
	}
	stats.LocksReleasedCount, _ = locksRes.RowsAffected()

	if err := tx.Commit(); err != nil {
		return stats, err
	}

	return stats, nil
}
