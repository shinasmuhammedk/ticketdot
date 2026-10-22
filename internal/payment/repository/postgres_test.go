package repository_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"ticketdot/internal/payment/model"
	"ticketdot/internal/payment/repository"
)

func setupTestDB(t *testing.T) *sqlx.DB {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("skipping database test: DATABASE_URL is not set")
	}
	db, err := sqlx.Connect("postgres", dbURL)
	if err != nil {
		t.Skipf("skipping database test: could not connect to DATABASE_URL: %v", err)
	}
	return db
}

func TestVerifyAndCaptureIntegration(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := repository.NewPostgresRepository(db)

	// Verify that querying a nonexistent order fails cleanly
	req := model.VerifyPaymentRequest{
		OrderID:   "order_nonexistent",
		PaymentID: "pay_test",
		Signature: "test_sig",
	}
	_, err := repo.VerifyAndCapture(ctx, "user-test", req, 50000)
	if err == nil {
		t.Fatal("expected error for nonexistent payment order, got nil")
	}
}
