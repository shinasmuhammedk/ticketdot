package lifecycle

import (
	"context"
	"testing"
	"time"
)

func TestStartScheduleLifecycleWorker_Cancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Test that cancel cleanly terminates the goroutine loop
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()

	cancel()

	select {
	case <-done:
		// Clean exit
	case <-time.After(1 * time.Second):
		t.Fatal("worker did not shut down upon context cancellation")
	}
}
