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

func (r *PostgresRepository) ConfirmBooking(userID string, req model.ConfirmBookingRequest) (*model.ConfirmBookingResponse, error) {
	cleaned := cleanSeats(req.Seats)
	if len(cleaned) == 0 {
		return nil, errors.New("at least one seat is required")
	}

	scheduleID := req.ScheduleID

	tx, err := r.db.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Get user's default details from users table if needed as fallback
	var userFallback struct {
		Name  string `db:"name"`
		Email string `db:"email"`
		Phone string `db:"phone"`
	}
	_ = tx.Get(&userFallback, "SELECT COALESCE(name, 'Passenger') as name, COALESCE(email, '') as email, COALESCE(phone, '') as phone FROM users WHERE id = $1", userID)

	// Build passenger map by seat number
	passengerMap := make(map[string]model.PassengerDetail)
	for _, p := range req.Passengers {
		sNum := strings.ToUpper(strings.TrimSpace(p.SeatNumber))
		if sNum != "" {
			passengerMap[sNum] = p
		}
	}

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

	var scheduleInfo struct {
		BaseFare    int64  `db:"base_fare"`
		Origin      string `db:"origin"`
		Destination string `db:"destination"`
	}
	err = tx.Get(&scheduleInfo, `
		SELECT s.base_fare, oc.name as origin, dc.name as destination
		FROM schedules s
		JOIN routes r ON s.route_id = r.id
		JOIN cities oc ON r.origin_id = oc.id
		JOIN cities dc ON r.destination_id = dc.id
		WHERE s.id = $1
	`, scheduleID)

	if err != nil {
		return nil, errors.New("schedule not found")
	}

	amount := scheduleInfo.BaseFare
	boardingPoint := strings.TrimSpace(req.BoardingPoint)
	if boardingPoint == "" {
		boardingPoint = scheduleInfo.Origin + " Central Bus Station"
	}
	droppingPoint := strings.TrimSpace(req.DroppingPoint)
	if droppingPoint == "" {
		droppingPoint = scheduleInfo.Destination + " Main Terminal"
	}

	contactPhone := strings.TrimSpace(req.ContactPhone)
	if contactPhone == "" {
		contactPhone = userFallback.Phone
	}
	contactEmail := strings.TrimSpace(req.ContactEmail)
	if contactEmail == "" {
		contactEmail = userFallback.Email
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

	var confirmedPassengers []model.PassengerDetail

	for _, seat := range cleaned {
		pName := userFallback.Name
		var pAge *int
		var pGender *string

		if p, ok := passengerMap[strings.ToUpper(seat)]; ok {
			if trimmed := strings.TrimSpace(p.Name); trimmed != "" {
				pName = trimmed
			}
			if p.Age > 0 {
				ageVal := p.Age
				pAge = &ageVal
			}
			if trimmedGender := strings.TrimSpace(p.Gender); trimmedGender != "" {
				genderVal := trimmedGender
				pGender = &genderVal
			}
		}

		ageForResp := 0
		if pAge != nil {
			ageForResp = *pAge
		}
		genderForResp := ""
		if pGender != nil {
			genderForResp = *pGender
		}

		confirmedPassengers = append(confirmedPassengers, model.PassengerDetail{
			SeatNumber: seat,
			Name:       pName,
			Age:        ageForResp,
			Gender:     genderForResp,
		})

		_, err = tx.Exec(`
			INSERT INTO bookings (
				booking_ref,
				user_id,
				schedule_id,
				seat_number,
				status,
				amount,
				passenger_name,
				passenger_age,
				passenger_gender,
				boarding_point,
				dropping_point,
				contact_phone,
				contact_email
			)
			VALUES ($1, $2, $3, $4, 'confirmed', $5, $6, $7, $8, $9, $10, $11, $12)
		`, bookingRef, userID, scheduleID, seat, amount, pName, pAge, pGender, boardingPoint, droppingPoint, contactPhone, contactEmail)

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
		BookingRef:    bookingRef,
		ScheduleID:    scheduleID,
		Seat:          strings.Join(cleaned, ", "),
		Seats:         cleaned,
		Passengers:    confirmedPassengers,
		BoardingPoint: boardingPoint,
		DroppingPoint: droppingPoint,
		ContactPhone:  contactPhone,
		ContactEmail:  contactEmail,
		Status:           "confirmed",
		TotalAmount:      amount * int64(len(cleaned)),
		TotalAmountPaise: amount * int64(len(cleaned)),
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
		BookingRef    string         `db:"booking_ref"`
		ScheduleID    string         `db:"schedule_id"`
		SeatNumber    string         `db:"seat_number"`
		Status        string         `db:"status"`
		Amount        int64          `db:"amount"`
		ConfirmedAt   time.Time      `db:"confirmed_at"`
		CreatedAt     time.Time      `db:"created_at"`
		Origin        string         `db:"origin"`
		Destination   string         `db:"destination"`
		OperatorName  string         `db:"operator_name"`
		BusNumber     string         `db:"bus_number"`
		BusType       string         `db:"bus_type"`
		DepartureTime string         `db:"departure_time"`
		ArrivalTime   string         `db:"arrival_time"`
		PaymentID     sql.NullString `db:"payment_id"`
		PaymentStatus sql.NullString `db:"payment_status"`
		RefundID      sql.NullString `db:"refund_id"`
		RefundStatus  sql.NullString `db:"refund_status"`
	}

	query := `
		SELECT
			b.booking_ref,
			b.schedule_id,
			b.seat_number,
			b.status,
			b.amount,
			b.confirmed_at,
			b.created_at,
			oc.name AS origin,
			dc.name AS destination,
			op.name AS operator_name,
			bus.registration_no AS bus_number,
			bus.bus_type AS bus_type,
			s.departure_time::text AS departure_time,
			s.arrival_time::text AS arrival_time,
			p.payment_id,
			p.status AS payment_status,
			p.refund_id,
			p.refund_status
		FROM bookings b
		JOIN schedules s ON b.schedule_id = s.id
		JOIN routes rt ON s.route_id = rt.id
		JOIN cities oc ON rt.origin_id = oc.id
		JOIN cities dc ON rt.destination_id = dc.id
		JOIN buses bus ON s.bus_id = bus.id
		JOIN operators op ON bus.operator_id = op.id
		LEFT JOIN payments p ON b.booking_ref = p.booking_ref
		WHERE b.user_id = $1
		ORDER BY b.created_at DESC, b.seat_number ASC
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
			result[idx].AmountPaise = result[idx].Amount
		} else {
			pID := ""
			if row.PaymentID.Valid {
				pID = row.PaymentID.String
			}
			pStatus := ""
			if row.PaymentStatus.Valid {
				pStatus = row.PaymentStatus.String
			}
			rID := ""
			if row.RefundID.Valid {
				rID = row.RefundID.String
			}
			rStatus := ""
			if row.RefundStatus.Valid {
				rStatus = row.RefundStatus.String
			}

			b := model.MyBooking{
				BookingRef:    row.BookingRef,
				ScheduleID:    row.ScheduleID,
				SeatNumber:    row.SeatNumber,
				Seat:          row.SeatNumber,
				Seats:         []string{row.SeatNumber},
				Origin:        row.Origin,
				Destination:   row.Destination,
				OperatorName:  row.OperatorName,
				BusNumber:     row.BusNumber,
				BusType:       row.BusType,
				DepartureTime: row.DepartureTime,
				ArrivalTime:   row.ArrivalTime,
				Status:        row.Status,
				Amount:        row.Amount,
				AmountPaise:   row.Amount,
				ConfirmedAt:   row.ConfirmedAt,
				PaymentID:     pID,
				PaymentStatus: pStatus,
				RefundID:      rID,
				RefundStatus:  rStatus,
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

	// Automated Refund Processing: Check if there was a captured payment for this booking
	var paymentInfo struct {
		Amount int64  `db:"amount"`
		Status string `db:"status"`
	}
	err = tx.Get(&paymentInfo, `
		SELECT amount, status
		FROM payments
		WHERE booking_ref = $1 AND status = 'captured'
		LIMIT 1
	`, bookingRef)

	refundID := ""
	var refundAmount int64
	refundStatus := ""
	refundTimeline := ""

	if err == nil && paymentInfo.Status == "captured" {
		randCandidate, _ := generateBookingRef()
		refundID = "rfnd_" + randCandidate
		refundAmount = paymentInfo.Amount
		refundStatus = "processed"
		refundTimeline = "Refund will be credited to original payment source within 5-7 business days"

		_, _ = tx.Exec(`
			UPDATE payments
			SET
				status = 'refunded',
				refund_id = $1,
				refund_amount = $2,
				refund_status = 'processed',
				refunded_at = NOW(),
				updated_at = NOW()
			WHERE booking_ref = $3
		`, refundID, refundAmount, bookingRef)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &model.CancelBookingResponse{
		BookingRef:        bookingRef,
		Status:            "cancelled",
		Message:           "booking cancelled successfully",
		Seats:             cancelledSeats,
		RefundID:          refundID,
		RefundAmount:      refundAmount,
		RefundAmountPaise: refundAmount,
		RefundStatus:      refundStatus,
		RefundTimeline:    refundTimeline,
	}, nil
}

func (r *PostgresRepository) GetBookingDetails(
	userID,
	bookingRef string,
) (*model.BookingDetailsResponse, error) {

	type detailRow struct {
		BookingRef      string         `db:"booking_ref"`
		SeatNumber      string         `db:"seat_number"`
		Status          string         `db:"status"`
		Amount          int64          `db:"amount"`
		PassengerName   string         `db:"passenger_name"`
		PassengerAge    sql.NullInt32  `db:"passenger_age"`
		PassengerGender sql.NullString `db:"passenger_gender"`
		BoardingPoint   sql.NullString `db:"boarding_point"`
		DroppingPoint   sql.NullString `db:"dropping_point"`
		ContactPhone    sql.NullString `db:"contact_phone"`
		ContactEmail    sql.NullString `db:"contact_email"`
		OperatorName    string         `db:"operator_name"`
		Origin          string         `db:"origin"`
		Destination     string         `db:"destination"`
		BusNumber       string         `db:"bus_number"`
		BusType         string         `db:"bus_type"`
		DepartureTime   string         `db:"departure_time"`
		ArrivalTime     string         `db:"arrival_time"`
	}

	query := `
		SELECT
			b.booking_ref,
			b.seat_number,
			b.status,
			b.amount,
			COALESCE(NULLIF(b.passenger_name, ''), u.name, 'Passenger') AS passenger_name,
			b.passenger_age,
			b.passenger_gender,
			b.boarding_point,
			b.dropping_point,
			b.contact_phone,
			b.contact_email,
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

		  AND (
		      b.user_id = NULLIF($2, '')::uuid
		      OR s.operator_id = (SELECT operator_id FROM users WHERE id = NULLIF($2, '')::uuid)
		      OR NULLIF($2, '') IS NULL
		  )
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
	var passengers []model.PassengerInfo
	var totalAmount int64
	for _, row := range rows {
		seats = append(seats, row.SeatNumber)
		totalAmount += row.Amount

		age := 0
		if row.PassengerAge.Valid {
			age = int(row.PassengerAge.Int32)
		}
		gender := ""
		if row.PassengerGender.Valid {
			gender = row.PassengerGender.String
		}

		passengers = append(passengers, model.PassengerInfo{
			SeatNumber: row.SeatNumber,
			Name:       row.PassengerName,
			Age:        age,
			Gender:     gender,
		})
	}

	first := rows[0]
	boardingPoint := first.Origin + " Central Bus Station"
	if first.BoardingPoint.Valid && strings.TrimSpace(first.BoardingPoint.String) != "" {
		boardingPoint = first.BoardingPoint.String
	}
	droppingPoint := first.Destination + " Main Terminal"
	if first.DroppingPoint.Valid && strings.TrimSpace(first.DroppingPoint.String) != "" {
		droppingPoint = first.DroppingPoint.String
	}
	contactPhone := ""
	if first.ContactPhone.Valid {
		contactPhone = first.ContactPhone.String
	}
	contactEmail := ""
	if first.ContactEmail.Valid {
		contactEmail = first.ContactEmail.String
	}

	resp := &model.BookingDetailsResponse{
		BookingRef:      first.BookingRef,
		PassengerName:   first.PassengerName,
		OperatorName:    first.OperatorName,
		Origin:          first.Origin,
		Destination:     first.Destination,
		BoardingPoint:   boardingPoint,
		DroppingPoint:   droppingPoint,
		ContactPhone:    contactPhone,
		ContactEmail:    contactEmail,
		BusNumber:       first.BusNumber,
		BusRegistration: first.BusNumber,
		BusType:         first.BusType,
		SeatNumber:      strings.Join(seats, ", "),
		Seats:           seats,
		Passengers:      passengers,
		DepartureTime:   first.DepartureTime,
		ArrivalTime:     first.ArrivalTime,
		Amount:          totalAmount,
		AmountPaise:     totalAmount,
		Status:          first.Status,
	}

	var payRow struct {
		PaymentID     sql.NullString `db:"payment_id"`
		PaymentMethod sql.NullString `db:"payment_method"`
		Status        sql.NullString `db:"status"`
		RefundID      sql.NullString `db:"refund_id"`
		RefundAmount  sql.NullInt64  `db:"refund_amount"`
		RefundStatus  sql.NullString `db:"refund_status"`
	}
	_ = r.db.Get(&payRow, `
		SELECT payment_id, payment_method, status, refund_id, refund_amount, refund_status
		FROM payments
		WHERE booking_ref = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, bookingRef)

	if payRow.PaymentID.Valid {
		resp.PaymentID = payRow.PaymentID.String
	}
	if payRow.PaymentMethod.Valid {
		resp.PaymentMethod = payRow.PaymentMethod.String
	}
	if payRow.Status.Valid {
		resp.PaymentStatus = payRow.Status.String
	}
	if payRow.RefundID.Valid {
		resp.RefundID = payRow.RefundID.String
	}
	if payRow.RefundAmount.Valid {
		resp.RefundAmount = payRow.RefundAmount.Int64
		resp.RefundAmountPaise = payRow.RefundAmount.Int64
	}
	if payRow.RefundStatus.Valid {
		resp.RefundStatus = payRow.RefundStatus.String
	}

	return resp, nil
}

