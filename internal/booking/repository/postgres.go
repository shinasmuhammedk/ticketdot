package repository

import (
	"database/sql"
	"errors"
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

func (r *PostgresRepository) LockSeat(userID, scheduleID, seat string) (*model.LockSeatResponse, error) {
	tx, err := r.db.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var currentStatus string

	err = tx.Get(
		&currentStatus,
		`
		SELECT status
		FROM seat_inventory
		WHERE schedule_id = $1
		  AND seat_number = $2
		FOR UPDATE
		`,
		scheduleID,
		seat,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("seat not found")
	}

	if err != nil {
		return nil, err
	}

	if currentStatus != "available" {
		return nil, errors.New("seat is not available")
	}

	lockExpires := time.Now().Add(5 * time.Minute)

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

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &model.LockSeatResponse{
		ScheduleID:  scheduleID,
		Seat:        seat,
		LockExpires: lockExpires.Format(time.RFC3339),
	}, nil
}

func (r *PostgresRepository) ConfirmBooking(userID, scheduleID, seat string) (*model.ConfirmBookingResponse, error) {
	tx, err := r.db.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var status string

	err = tx.Get(&status, `
		SELECT status
		FROM seat_inventory
		WHERE schedule_id = $1
		  AND seat_number = $2
		  AND locked_by = $3
		FOR UPDATE
	`, scheduleID, seat, userID)

	if err != nil {
		return nil, errors.New("seat lock not found for this user")
	}

	if status != "locked" {
		return nil, errors.New("seat is not locked")
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

	bookingRef := "TD-" + time.Now().Format("20060102150405")

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
		SET status = 'booked'
		WHERE schedule_id = $1
		  AND seat_number = $2
	`, scheduleID, seat)

	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &model.ConfirmBookingResponse{
		BookingRef: bookingRef,
		ScheduleID: scheduleID,
		Seat:       seat,
		Status:     "confirmed",
	}, nil
}

func (r *PostgresRepository) GetMyBookings(userID string) ([]model.MyBooking, error) {
	query := `
		SELECT
			booking_ref,
			schedule_id,
			seat_number,
			status,
			amount,
			confirmed_at
		FROM bookings
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	bookings := []model.MyBooking{}

	err := r.db.Select(&bookings, query, userID)
	if err != nil {
		return nil, err
	}

	return bookings, nil
}

func (r *PostgresRepository) CancelBooking(userID, bookingRef string) (*model.CancelBookingResponse, error) {
	tx, err := r.db.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var booking struct {
		ScheduleID string `db:"schedule_id"`
		SeatNumber string `db:"seat_number"`
		Status     string `db:"status"`
	}

	err = tx.Get(&booking, `
		SELECT schedule_id, seat_number, status
		FROM bookings
		WHERE booking_ref = $1
		  AND user_id = $2
		FOR UPDATE
	`, bookingRef, userID)

	if err != nil {
		return nil, errors.New("booking not found")
	}

	if booking.Status != "confirmed" {
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

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &model.CancelBookingResponse{
		BookingRef: bookingRef,
		Status:     "cancelled",
		Message:    "booking cancelled successfully",
	}, nil
}

func (r *PostgresRepository) GetBookingDetails(
	userID,
	bookingRef string,
) (*model.BookingDetailsResponse, error) {

	var booking model.BookingDetailsResponse

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

JOIN users u
    ON b.user_id = u.id

JOIN schedules s
    ON b.schedule_id = s.id

JOIN routes rt
    ON s.route_id = rt.id

JOIN cities oc
    ON rt.origin_id = oc.id

JOIN cities dc
    ON rt.destination_id = dc.id

JOIN buses bus
    ON s.bus_id = bus.id

JOIN operators op
    ON bus.operator_id = op.id

WHERE b.booking_ref = $1
AND b.user_id = $2
	`

	err := r.db.Get(
		&booking,
		query,
		bookingRef,
		userID,
	)

	if err != nil {
		return nil, err
	}

	return &booking, nil
}
