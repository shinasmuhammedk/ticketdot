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
		dbURL = "postgres://postgres:Shinas@localhost:5432/BusTicket?sslmode=disable"
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
