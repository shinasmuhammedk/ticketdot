package repository

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"ticketdot/internal/booking/model"
)

type PostgresRepository struct {
	db *sqlx.DB
}

func NewPostgresRepository(db *sqlx.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

var _ BookingRepository = (*PostgresRepository)(nil)

const bookingRefChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func generateBookingRef() (string, error) {
	bytes := make([]byte, 8)
	for i := range bytes {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(bookingRefChars))))
		if err != nil {
			return "", err
		}
		bytes[i] = bookingRefChars[num.Int64()]
	}
	return string(bytes), nil
}

func cleanSeats(seats []string) []string {
	seen := make(map[string]bool)
	var cleaned []string
	for _, s := range seats {
		s = strings.ToUpper(strings.TrimSpace(s))
		if s != "" && !seen[s] {
			seen[s] = true
			cleaned = append(cleaned, s)
		}
	}
	sort.Strings(cleaned)
	return cleaned
}

func (r *PostgresRepository) LockSeats(userID, scheduleID string, seats []string) (*model.LockSeatResponse, error) {
	cleaned := cleanSeats(seats)
	if len(cleaned) == 0 {
		return nil, errors.New("at least one seat is required")
	}
	if len(cleaned) > 6 {
		return nil, errors.New("cannot lock more than 6 seats")
	}

	tx, err := r.db.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var scheduleInfo struct {
		HasDeparted bool `db:"has_departed"`
	}
	err = tx.Get(&scheduleInfo, `
		SELECT (departure_time <= NOW()) AS has_departed
		FROM schedules
		WHERE id = $1
	`, scheduleID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("schedule not found")
	}
	if err != nil {
		return nil, err
	}
	if scheduleInfo.HasDeparted {
		return nil, errors.New("schedule has already departed")
	}

	// Verify all seats in sorted order
	for _, seat := range cleaned {
		var seatInfo struct {
			Status        string `db:"status"`
			IsExpiredLock bool   `db:"is_expired_lock"`
		}

		err = tx.Get(
			&seatInfo,
			`
			SELECT
				status,
				(status = 'locked' AND lock_expires IS NOT NULL AND lock_expires < NOW()) AS is_expired_lock
			FROM seat_inventory
			WHERE schedule_id = $1
			  AND seat_number = $2
			FOR UPDATE
			`,
			scheduleID,
			seat,
		)

		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("seat %s not found", seat)
		}
		if err != nil {
			return nil, err
		}

		isAvailable := seatInfo.Status == "available" || seatInfo.IsExpiredLock
		if !isAvailable {
			return nil, fmt.Errorf("seat %s is not available", seat)
		}
	}

	lockExpires := time.Now().Add(5 * time.Minute)

	// Lock all seats
	for _, seat := range cleaned {
		_, err = tx.Exec(
			`
			UPDATE seat_inventory
			SET
				status = 'locked',
				locked_by = $1,
				locked_at = NOW(),
				lock_expires = $2
			WHERE schedule_id = $3
			  AND seat_number = $4
			`,
			userID,
			lockExpires,
			scheduleID,
			seat,
		)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &model.LockSeatResponse{
		ScheduleID:  scheduleID,
		Seat:        strings.Join(cleaned, ", "),
		Seats:       cleaned,
		LockExpires: lockExpires.Format(time.RFC3339),
	}, nil
}

func (r *PostgresRepository) ConfirmBooking(userID, scheduleID string, seats []string) (*model.ConfirmBookingResponse, error) {
	cleaned := cleanSeats(seats)
	if len(cleaned) == 0 {
		return nil, errors.New("at least one seat is required")
	}

	tx, err := r.db.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Verify all seats locked by this user and not expired
	for _, seat := range cleaned {
		var lockInfo struct {
			Status    string `db:"status"`
			IsExpired bool   `db:"is_expired"`
		}

		err = tx.Get(&lockInfo, `
			SELECT
				status,
				(lock_expires IS NULL OR lock_expires < NOW()) AS is_expired
			FROM seat_inventory
			WHERE schedule_id = $1
			  AND seat_number = $2
			  AND locked_by = $3
			FOR UPDATE
		`, scheduleID, seat, userID)

		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("seat %s lock not found for this user", seat)
		}
		if err != nil {
			return nil, err
		}

		if lockInfo.Status != "locked" {
			return nil, fmt.Errorf("seat %s is not locked", seat)
		}

		if lockInfo.IsExpired {
			return nil, fmt.Errorf("seat %s lock has expired", seat)
		}
	}

	var amount float64
	err = tx.Get(&amount, `
		SELECT base_fare
		FROM schedules
		WHERE id = $1
	`, scheduleID)

	if err != nil {
		return nil, errors.New("schedule not found")
	}

	var bookingRef string
	for attempt := 0; attempt < 10; attempt++ {
		candidate, err := generateBookingRef()
		if err != nil {
			return nil, fmt.Errorf("failed to generate booking ref: %w", err)
		}
		var exists bool
		err = tx.Get(&exists, "SELECT EXISTS(SELECT 1 FROM bookings WHERE booking_ref = $1)", candidate)
		if err != nil {
			return nil, err
		}
		if !exists {
			bookingRef = candidate
			break
		}
	}
	if bookingRef == "" {
		return nil, errors.New("failed to generate unique booking reference")
	}

	for _, seat := range cleaned {
		_, err = tx.Exec(`
			INSERT INTO bookings (
				booking_ref,
				user_id,
				schedule_id,
				seat_number,
				status,
				amount
			)
			VALUES ($1, $2, $3, $4, 'confirmed', $5)
		`, bookingRef, userID, scheduleID, seat, amount)

		if err != nil {
			return nil, err
		}

		_, err = tx.Exec(`
			UPDATE seat_inventory
			SET
				status = 'booked',
				locked_by = NULL,
				locked_at = NULL,
				lock_expires = NULL
			WHERE schedule_id = $1
			  AND seat_number = $2
		`, scheduleID, seat)

		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &model.ConfirmBookingResponse{
		BookingRef:  bookingRef,
		ScheduleID:  scheduleID,
		Seat:        strings.Join(cleaned, ", "),
		Seats:       cleaned,
		Status:      "confirmed",
		TotalAmount: amount * float64(len(cleaned)),
	}, nil
}

func (r *PostgresRepository) ReleaseSeats(userID, scheduleID string, seats []string) (*model.ReleaseSeatsResponse, error) {
	cleaned := cleanSeats(seats)
	if len(cleaned) == 0 {
		return nil, errors.New("at least one seat is required")
	}

	tx, err := r.db.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	for _, seat := range cleaned {
		_, err = tx.Exec(`
			UPDATE seat_inventory
			SET
				status = 'available',
				locked_by = NULL,
				locked_at = NULL,
				lock_expires = NULL
			WHERE schedule_id = $1
			  AND seat_number = $2
			  AND locked_by = $3
			  AND status = 'locked'
		`, scheduleID, seat, userID)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &model.ReleaseSeatsResponse{
		Message: "locks released successfully",
		Seats:   cleaned,
	}, nil
}

func (r *PostgresRepository) GetMyBookings(userID string) ([]model.MyBooking, error) {
	type bookingRow struct {
		BookingRef  string    `db:"booking_ref"`
		ScheduleID  string    `db:"schedule_id"`
		SeatNumber  string    `db:"seat_number"`
		Status      string    `db:"status"`
		Amount      float64   `db:"amount"`
		ConfirmedAt time.Time `db:"confirmed_at"`
		CreatedAt   time.Time `db:"created_at"`
	}

	query := `
		SELECT
			booking_ref,
			schedule_id,
			seat_number,
			status,
			amount,
			confirmed_at,
			created_at
		FROM bookings
		WHERE user_id = $1
		ORDER BY created_at DESC, seat_number ASC
	`

	var rows []bookingRow
	err := r.db.Select(&rows, query, userID)
	if err != nil {
		return nil, err
	}

	var result []model.MyBooking
	indexMap := make(map[string]int)

	for _, row := range rows {
		idx, exists := indexMap[row.BookingRef]
		if exists {
			result[idx].Seats = append(result[idx].Seats, row.SeatNumber)
			result[idx].SeatNumber = strings.Join(result[idx].Seats, ", ")
			result[idx].Seat = result[idx].SeatNumber
			result[idx].Amount += row.Amount
		} else {
			b := model.MyBooking{
				BookingRef:  row.BookingRef,
				ScheduleID:  row.ScheduleID,
				SeatNumber:  row.SeatNumber,
				Seat:        row.SeatNumber,
				Seats:       []string{row.SeatNumber},
				Status:      row.Status,
				Amount:      row.Amount,
				ConfirmedAt: row.ConfirmedAt,
			}
			indexMap[row.BookingRef] = len(result)
			result = append(result, b)
		}
	}

	return result, nil
}

func (r *PostgresRepository) CancelBooking(userID, bookingRef string) (*model.CancelBookingResponse, error) {
	tx, err := r.db.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	type bookingRow struct {
		ScheduleID string `db:"schedule_id"`
		SeatNumber string `db:"seat_number"`
		Status     string `db:"status"`
	}

	var bookings []bookingRow
	err = tx.Select(&bookings, `
		SELECT schedule_id, seat_number, status
		FROM bookings
		WHERE booking_ref = $1
		  AND user_id = $2
		FOR UPDATE
	`, bookingRef, userID)

	if err != nil {
		return nil, err
	}

	if len(bookings) == 0 {
		return nil, errors.New("booking not found")
	}

	allCancelled := true
	for _, b := range bookings {
		if b.Status == "confirmed" {
			allCancelled = false
			break
		}
	}
	if allCancelled {
		return nil, errors.New("only confirmed bookings can be cancelled")
	}

	_, err = tx.Exec(`
		UPDATE bookings
		SET status = 'cancelled'
		WHERE booking_ref = $1
		  AND user_id = $2
	`, bookingRef, userID)

	if err != nil {
		return nil, err
	}

	var cancelledSeats []string
	for _, booking := range bookings {
		cancelledSeats = append(cancelledSeats, booking.SeatNumber)
		_, err = tx.Exec(`
			UPDATE seat_inventory
			SET
				status = 'available',
				locked_by = NULL,
				locked_at = NULL,
				lock_expires = NULL
			WHERE schedule_id = $1
			  AND seat_number = $2
		`, booking.ScheduleID, booking.SeatNumber)

		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &model.CancelBookingResponse{
		BookingRef: bookingRef,
		Status:     "cancelled",
		Message:    "booking cancelled successfully",
		Seats:      cancelledSeats,
	}, nil
}

func (r *PostgresRepository) GetBookingDetails(
	userID,
	bookingRef string,
) (*model.BookingDetailsResponse, error) {

	type detailRow struct {
		BookingRef    string  `db:"booking_ref"`
		SeatNumber    string  `db:"seat_number"`
		Status        string  `db:"status"`
		Amount        float64 `db:"amount"`
		PassengerName string  `db:"passenger_name"`
		OperatorName  string  `db:"operator_name"`
		Origin        string  `db:"origin"`
		Destination   string  `db:"destination"`
		BusNumber     string  `db:"bus_number"`
		BusType       string  `db:"bus_type"`
		DepartureTime string  `db:"departure_time"`
		ArrivalTime   string  `db:"arrival_time"`
	}

	query := `
		SELECT
			b.booking_ref,
			b.seat_number,
			b.status,
			b.amount,
			u.name AS passenger_name,
			op.name AS operator_name,
			oc.name AS origin,
			dc.name AS destination,
			bus.registration_no AS bus_number,
			bus.bus_type AS bus_type,
			s.departure_time::text AS departure_time,
			s.arrival_time::text AS arrival_time
		FROM bookings b
		JOIN users u ON b.user_id = u.id
		JOIN schedules s ON b.schedule_id = s.id
		JOIN routes rt ON s.route_id = rt.id
		JOIN cities oc ON rt.origin_id = oc.id
		JOIN cities dc ON rt.destination_id = dc.id
		JOIN buses bus ON s.bus_id = bus.id
		JOIN operators op ON bus.operator_id = op.id
		WHERE b.booking_ref = $1
		  AND b.user_id = $2
		ORDER BY b.seat_number ASC
	`

	var rows []detailRow
	err := r.db.Select(&rows, query, bookingRef, userID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, errors.New("booking not found")
	}

	var seats []string
	var totalAmount float64
	for _, row := range rows {
		seats = append(seats, row.SeatNumber)
		totalAmount += row.Amount
	}

	first := rows[0]
	resp := &model.BookingDetailsResponse{
		BookingRef:      first.BookingRef,
		PassengerName:   first.PassengerName,
		OperatorName:    first.OperatorName,
		Origin:          first.Origin,
		Destination:     first.Destination,
		BusNumber:       first.BusNumber,
		BusRegistration: first.BusNumber,
		BusType:         first.BusType,
		SeatNumber:      strings.Join(seats, ", "),
		Seats:           seats,
		DepartureTime:   first.DepartureTime,
		ArrivalTime:     first.ArrivalTime,
		Amount:          totalAmount,
		Status:          first.Status,
	}

	return resp, nil
}
