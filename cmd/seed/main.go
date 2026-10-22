package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"

	"ticketdot/migrations"
)

func main() {
	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dbURL == "" {
		log.Fatal("ERROR: DATABASE_URL environment variable is required")
	}

	if !strings.Contains(strings.ToLower(dbURL), "test") {
		log.Fatalf("ABORTING: DATABASE_URL must contain 'test' to prevent seeding non-test databases. Received: %s", dbURL)
	}

	db, err := sqlx.Connect("postgres", dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to test database: %v", err)
	}
	defer db.Close()

	log.Println("Applying database migrations on test database...")
	if err := migrations.Run(db); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	log.Println("Seeding test database...")

	// 1. Seed Cities
	cities := []struct {
		name  string
		state string
	}{
		{"Mumbai", "Maharashtra"},
		{"Pune", "Maharashtra"},
		{"Bangalore", "Karnataka"},
		{"Hyderabad", "Telangana"},
	}

	cityIDs := make(map[string]string)
	for _, c := range cities {
		var id string
		err := db.QueryRow(`
			INSERT INTO cities (name, state)
			VALUES ($1, $2)
			ON CONFLICT (name) DO UPDATE SET state = EXCLUDED.state
			RETURNING id
		`, c.name, c.state).Scan(&id)
		if err != nil {
			log.Fatalf("Failed to seed city %s: %v", c.name, err)
		}
		cityIDs[c.name] = id
	}

	// Password hash for 'Password123!' with bcrypt cost 12
	pwHashBytes, err := bcrypt.GenerateFromPassword([]byte("Password123!"), 12)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}
	pwHash := string(pwHashBytes)

	// 2. Seed Operator User & Operator Entity
	var operatorID string
	err = db.QueryRow(`
		INSERT INTO operators (name)
		VALUES ('Test Express Lines')
		RETURNING id
	`).Scan(&operatorID)
	if err != nil {
		// If table exists, try fetching
		_ = db.Get(&operatorID, `SELECT id FROM operators WHERE name = 'Test Express Lines' LIMIT 1`)
	}

	var operatorUserID string
	err = db.QueryRow(`
		INSERT INTO users (name, email, phone, password_hash, role, operator_id, email_verified)
		VALUES ('Test Operator', 'operator@test.com', '9876543210', $1, 'operator_admin', $2, TRUE)
		ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash, operator_id = EXCLUDED.operator_id
		RETURNING id
	`, pwHash, operatorID).Scan(&operatorUserID)
	if err != nil {
		log.Fatalf("Failed to seed operator user: %v", err)
	}

	// 3. Seed Passenger User
	var passengerUserID string
	err = db.QueryRow(`
		INSERT INTO users (name, email, phone, password_hash, role, email_verified)
		VALUES ('Test Passenger', 'passenger@test.com', '9123456789', $1, 'passenger', TRUE)
		ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash
		RETURNING id
	`, pwHash).Scan(&passengerUserID)
	if err != nil {
		log.Fatalf("Failed to seed passenger user: %v", err)
	}

	// 4. Seed Route (Mumbai -> Pune)
	mumbaiID := cityIDs["Mumbai"]
	puneID := cityIDs["Pune"]
	var routeID string
	err = db.Get(&routeID, `SELECT id FROM routes WHERE origin_id = $1 AND destination_id = $2 LIMIT 1`, mumbaiID, puneID)
	if err != nil || routeID == "" {
		err = db.QueryRow(`
			INSERT INTO routes (origin_id, destination_id)
			VALUES ($1, $2)
			RETURNING id
		`, mumbaiID, puneID).Scan(&routeID)
		if err != nil {
			log.Fatalf("Failed to seed route: %v", err)
		}
	}

	// 5. Seed Bus
	var busID string
	err = db.Get(&busID, `SELECT id FROM buses WHERE registration_no = 'MH-01-TEST-101' LIMIT 1`)
	if err != nil || busID == "" {
		err = db.QueryRow(`
			INSERT INTO buses (operator_id, bus_type, total_seats, registration_no)
			VALUES ($1, 'AC_SLEEPER', 36, 'MH-01-TEST-101')
			RETURNING id
		`, operatorID).Scan(&busID)
		if err != nil {
			log.Fatalf("Failed to seed bus: %v", err)
		}

		// Seed physical seat layouts for the bus
		for r := 1; r <= 9; r++ {
			for c := 1; c <= 4; c++ {
				seatNum := fmt.Sprintf("%c%d", 'A'+(c-1), r)
				_, _ = db.Exec(`
					INSERT INTO seat_layouts (bus_id, seat_number, seat_type, deck_type, row_num, col_num, is_window)
					VALUES ($1, $2, 'sleeper', 'lower', $3, $4, $5)
					ON CONFLICT (bus_id, seat_number) DO NOTHING
				`, busID, seatNum, r, c, (c == 1 || c == 4))
			}
		}
	}

	// 6. Seed Schedule (Tomorrow 08:00 - 12:00)
	tomorrow := time.Now().Add(24 * time.Hour)
	depTime := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 8, 0, 0, 0, time.UTC)
	arrTime := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 12, 0, 0, 0, time.UTC)

	var schedID string
	err = db.Get(&schedID, `
		SELECT id FROM schedules 
		WHERE route_id = $1 AND bus_id = $2 AND status = 'scheduled' AND departure_time > NOW()
		LIMIT 1
	`, routeID, busID)
	if err != nil || schedID == "" {
		err = db.QueryRow(`
			INSERT INTO schedules (route_id, bus_id, operator_id, departure_time, arrival_time, available_seats, base_fare, status)
			VALUES ($1, $2, $3, $4, $5, 36, 49900, 'scheduled')
			RETURNING id
		`, routeID, busID, operatorID, depTime, arrTime).Scan(&schedID)
		if err != nil {
			log.Fatalf("Failed to seed schedule: %v", err)
		}

		// Seed seat inventory for this schedule
		for r := 1; r <= 9; r++ {
			for c := 1; c <= 4; c++ {
				seatNum := fmt.Sprintf("%c%d", 'A'+(c-1), r)
				_, _ = db.Exec(`
					INSERT INTO seat_inventory (schedule_id, seat_number, status)
					VALUES ($1, $2, 'available')
					ON CONFLICT (schedule_id, seat_number) DO NOTHING
				`, schedID, seatNum)
			}
		}
	}

	log.Println("SUCCESS: Test database seeded successfully!")
	log.Printf("Passenger: passenger@test.com / Password123! (ID: %s)\n", passengerUserID)
	log.Printf("Operator:  operator@test.com / Password123! (ID: %s, Operator: %s)\n", operatorUserID, operatorID)
	log.Printf("Schedule:  Mumbai -> Pune at %s (ID: %s)\n", depTime.Format(time.RFC3339), schedID)
}
