package migrations_test

import (
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"ticketdot/migrations"
)

func TestMigrationsRun(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:Shinas@localhost:5432/busticket_test?sslmode=disable"
	}

	db, err := sqlx.Connect("postgres", dbURL)
	if err != nil {
		t.Skipf("skipping db test: postgres not accessible (%v)", err)
	}
	defer db.Close()

	// 1. Run migrations
	if err := migrations.Run(db); err != nil {
		t.Fatalf("migrations.Run failed: %v", err)
	}

	// 2. Verify schema_migrations table has records
	var count int
	if err := db.Get(&count, "SELECT COUNT(*) FROM schema_migrations"); err != nil {
		t.Fatalf("failed to query schema_migrations: %v", err)
	}
	if count < 5 {
		t.Errorf("expected at least 5 migrations applied, got %d", count)
	}

	// 3. Second run must be a no-op and succeed idempotently
	if err := migrations.Run(db); err != nil {
		t.Fatalf("second migrations.Run failed: %v", err)
	}
}

func TestMigration017DataPreservation(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:Shinas@localhost:5432/busticket_test?sslmode=disable"
	}

	db, err := sqlx.Connect("postgres", dbURL)
	if err != nil {
		t.Skipf("skipping db test: postgres not accessible (%v)", err)
	}
	defer db.Close()

	// Ensure all migrations up to 017 are applied
	if err := migrations.Run(db); err != nil {
		t.Fatalf("migrations.Run failed: %v", err)
	}

	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Test data preservation conversion logic on a table simulating pre-migration schema with NUMERIC(10,2) values
	_, err = tx.Exec(`
		CREATE TEMPORARY TABLE test_money_migration_pre (
			id SERIAL PRIMARY KEY,
			base_fare NUMERIC(10,2) NOT NULL,
			amount NUMERIC(10,2) NOT NULL,
			refund_amount NUMERIC(10,2) DEFAULT 0
		) ON COMMIT DROP;
	`)
	if err != nil {
		t.Fatalf("failed to create temporary table: %v", err)
	}

	// Insert test rows with various decimal rupee amounts
	testValues := []struct {
		fare         float64
		amount       float64
		refund       float64
		expectedFare int64
		expectedAmt  int64
		expectedRef  int64
	}{
		{450.00, 900.00, 0.00, 45000, 90000, 0},
		{499.50, 1498.50, 499.50, 49950, 149850, 49950},
		{123.45, 246.90, 123.45, 12345, 24690, 12345},
		{0.01, 0.02, 0.01, 1, 2, 1},
		{99999.99, 99999.99, 99999.99, 9999999, 9999999, 9999999},
	}

	for _, tv := range testValues {
		_, err := tx.Exec(`
			INSERT INTO test_money_migration_pre (base_fare, amount, refund_amount)
			VALUES ($1, $2, $3)
		`, tv.fare, tv.amount, tv.refund)
		if err != nil {
			t.Fatalf("failed to insert test row: %v", err)
		}
	}

	// Apply identical Migration 017 conversion
	_, err = tx.Exec(`
		ALTER TABLE test_money_migration_pre
			ALTER COLUMN base_fare TYPE BIGINT USING ROUND(base_fare * 100),
			ALTER COLUMN amount TYPE BIGINT USING ROUND(amount * 100),
			ALTER COLUMN refund_amount TYPE BIGINT USING ROUND(COALESCE(refund_amount, 0) * 100);
	`)
	if err != nil {
		t.Fatalf("failed to execute migration 017 conversion on populated table: %v", err)
	}

	// Assert that values before * 100 equal values after
	rows, err := tx.Query(`
		SELECT id, base_fare, amount, refund_amount
		FROM test_money_migration_pre
		ORDER BY id ASC
	`)
	if err != nil {
		t.Fatalf("failed to query migrated table: %v", err)
	}
	defer rows.Close()

	idx := 0
	for rows.Next() {
		var id int
		var fare, amount, refund int64
		if err := rows.Scan(&id, &fare, &amount, &refund); err != nil {
			t.Fatalf("failed to scan migrated row: %v", err)
		}
		expected := testValues[idx]
		if fare != expected.expectedFare {
			t.Errorf("row %d fare: got %d paise, want %d (orig: %v)", id, fare, expected.expectedFare, expected.fare)
		}
		if amount != expected.expectedAmt {
			t.Errorf("row %d amount: got %d paise, want %d (orig: %v)", id, amount, expected.expectedAmt, expected.amount)
		}
		if refund != expected.expectedRef {
			t.Errorf("row %d refund: got %d paise, want %d (orig: %v)", id, refund, expected.expectedRef, expected.refund)
		}
		idx++
	}

	// Also verify that actual schema columns are now BIGINT
	var baseFareType, amountType, refundAmountType string
	err = db.QueryRow(`
		SELECT data_type FROM information_schema.columns
		WHERE table_name = 'schedules' AND column_name = 'base_fare'
	`).Scan(&baseFareType)
	if err != nil {
		t.Fatalf("failed to check schedules.base_fare data_type: %v", err)
	}
	if baseFareType != "bigint" {
		t.Errorf("schedules.base_fare column type = %s, want bigint", baseFareType)
	}

	err = db.QueryRow(`
		SELECT data_type FROM information_schema.columns
		WHERE table_name = 'bookings' AND column_name = 'amount'
	`).Scan(&amountType)
	if err != nil {
		t.Fatalf("failed to check bookings.amount data_type: %v", err)
	}
	if amountType != "bigint" {
		t.Errorf("bookings.amount column type = %s, want bigint", amountType)
	}

	err = db.QueryRow(`
		SELECT data_type FROM information_schema.columns
		WHERE table_name = 'payments' AND column_name = 'amount'
	`).Scan(&amountType)
	if err != nil {
		t.Fatalf("failed to check payments.amount data_type: %v", err)
	}
	if amountType != "bigint" {
		t.Errorf("payments.amount column type = %s, want bigint", amountType)
	}

	err = db.QueryRow(`
		SELECT data_type FROM information_schema.columns
		WHERE table_name = 'payments' AND column_name = 'refund_amount'
	`).Scan(&refundAmountType)
	if err != nil {
		t.Fatalf("failed to check payments.refund_amount data_type: %v", err)
	}
	if refundAmountType != "bigint" {
		t.Errorf("payments.refund_amount column type = %s, want bigint", refundAmountType)
	}
}
