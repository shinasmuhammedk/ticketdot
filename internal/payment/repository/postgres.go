package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"ticketdot/internal/httpx"
	"ticketdot/internal/payment/model"
)

type PostgresRepository struct {
	db *sqlx.DB
}

func NewPostgresRepository(db *sqlx.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func generateBookingRef() (string, error) {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 8)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		b[i] = charset[n.Int64()]
	}
	return string(b), nil
}

func (r *PostgresRepository) GetScheduleFare(ctx context.Context, scheduleID string) (int64, string, string, error) {
	var info struct {
		BaseFare    int64  `db:"base_fare"`
		Origin      string `db:"origin"`
		Destination string `db:"destination"`
	}

	err := r.db.GetContext(ctx, &info, `
		SELECT s.base_fare, oc.name as origin, dc.name as destination
		FROM schedules s
		JOIN routes r ON s.route_id = r.id
		JOIN cities oc ON r.origin_id = oc.id
		JOIN cities dc ON r.destination_id = dc.id
		WHERE s.id = $1
	`, scheduleID)

	if err != nil {
		return 0, "", "", errors.New("schedule not found")
	}

	return info.BaseFare, info.Origin, info.Destination, nil
}

func (r *PostgresRepository) ValidateSeatLocks(ctx context.Context, scheduleID string, seats []string, userID string) error {
	for _, seat := range seats {
		var status string
		var isExpired bool
		err := r.db.QueryRowContext(ctx, `
			SELECT
				status,
				(lock_expires IS NULL OR lock_expires < NOW()) AS is_expired
			FROM seat_inventory
			WHERE schedule_id = $1
			  AND seat_number = $2
			  AND locked_by = $3
		`, scheduleID, seat, userID).Scan(&status, &isExpired)

		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("seat %s is not held by your session", seat)
		}
		if err != nil {
			return err
		}
		if status != "locked" {
			return fmt.Errorf("seat %s is not currently locked", seat)
		}
		if isExpired {
			return fmt.Errorf("seat %s lock has expired. Please reselect seats", seat)
		}
	}
	return nil
}

func (r *PostgresRepository) CreateOrder(ctx context.Context, p *model.Payment) error {
	query := `
		INSERT INTO payments (
			order_id,
			user_id,
			schedule_id,
			seats,
			amount,
			currency,
			status,
			gateway,
			expires_at,
			created_at,
			updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW()
		) RETURNING id, created_at, updated_at
	`
	return r.db.QueryRowContext(
		ctx,
		query,
		p.OrderID,
		p.UserID,
		p.ScheduleID,
		pq.Array(p.Seats),
		p.Amount,
		p.Currency,
		p.Status,
		p.Gateway,
		p.ExpiresAt,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
}

func (r *PostgresRepository) CreateOrderAndExtendLocks(ctx context.Context, p *model.Payment, windowMinutes int) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if windowMinutes <= 0 {
		windowMinutes = 12
	}

	// 0. Expire any stale pending orders on this schedule so expired orders do not hold back seats or trigger uniqueness conflicts
	_, err = tx.ExecContext(ctx, `
		UPDATE payments
		SET status = 'expired', updated_at = NOW()
		WHERE schedule_id = $1
		  AND status = 'pending'
		  AND expires_at IS NOT NULL
		  AND expires_at < NOW()
	`, p.ScheduleID)
	if err != nil {
		return fmt.Errorf("failed to clean expired pending orders: %w", err)
	}

	// 1. Lock seat inventory rows FOR UPDATE to prevent concurrent modifications
	var seatRows []struct {
		SeatNumber  string     `db:"seat_number"`
		Status      string     `db:"status"`
		LockedBy    *string    `db:"locked_by"`
		LockExpires *time.Time `db:"lock_expires"`
	}

	err = tx.SelectContext(ctx, &seatRows, `
		SELECT seat_number, status, locked_by, lock_expires
		FROM seat_inventory
		WHERE schedule_id = $1
		  AND seat_number = ANY($2)
		ORDER BY seat_number ASC
		FOR UPDATE
	`, p.ScheduleID, pq.Array(p.Seats))
	if err != nil {
		return fmt.Errorf("failed to lock seat rows: %w", err)
	}

	if len(seatRows) != len(p.Seats) {
		return httpx.Validation("one or more requested seats do not exist")
	}

	now := time.Now()
	for _, sr := range seatRows {
		if sr.Status != "locked" || sr.LockedBy == nil || *sr.LockedBy != p.UserID {
			return httpx.Validation(fmt.Sprintf("seat %s is not held by your session", sr.SeatNumber))
		}
		if sr.LockExpires == nil || sr.LockExpires.Before(now) {
			return httpx.Validation(fmt.Sprintf("seat %s lock has expired. Please reselect seats", sr.SeatNumber))
		}
	}

	// 1.5 Overlap protection: check for any other user's pending, unexpired payment on the same schedule whose seats overlap
	var overlapCount int
	err = tx.GetContext(ctx, &overlapCount, `
		SELECT COUNT(*)
		FROM payments
		WHERE schedule_id = $1
		  AND user_id != $2
		  AND status = 'pending'
		  AND (expires_at IS NULL OR expires_at > NOW())
		  AND seats && $3::text[]
	`, p.ScheduleID, p.UserID, pq.Array(p.Seats))
	if err != nil {
		return fmt.Errorf("failed to check overlapping orders: %w", err)
	}
	if overlapCount > 0 {
		return httpx.Conflict("a pending payment order already exists for one or more of these seats")
	}

	// 2. Extend seat locks: GREATEST(lock_expires, NOW() + INTERVAL '1 minute' * windowMinutes)
	_, err = tx.ExecContext(ctx, `
		UPDATE seat_inventory
		SET lock_expires = GREATEST(lock_expires, NOW() + ($1 * INTERVAL '1 minute'))
		WHERE schedule_id = $2
		  AND seat_number = ANY($3)
		  AND status = 'locked'
		  AND locked_by = $4
	`, windowMinutes, p.ScheduleID, pq.Array(p.Seats), p.UserID)
	if err != nil {
		return fmt.Errorf("failed to extend seat locks: %w", err)
	}

	// 3. Insert payment record with expires_at
	query := `
		INSERT INTO payments (
			order_id,
			user_id,
			schedule_id,
			seats,
			amount,
			currency,
			status,
			gateway,
			expires_at,
			created_at,
			updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, 'pending', $7, NOW() + ($8 * INTERVAL '1 minute'), NOW(), NOW()
		) RETURNING id, expires_at, created_at, updated_at
	`
	err = tx.QueryRowContext(
		ctx,
		query,
		p.OrderID,
		p.UserID,
		p.ScheduleID,
		pq.Array(p.Seats),
		p.Amount,
		p.Currency,
		p.Gateway,
		windowMinutes,
	).Scan(&p.ID, &p.ExpiresAt, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return httpx.Conflict("a pending payment order already exists for these seats")
		}
		return fmt.Errorf("failed to save payment order: %w", err)
	}

	return tx.Commit()
}

func (r *PostgresRepository) FindPendingOrderByScheduleAndSeats(ctx context.Context, scheduleID string, seats []string) (*model.Payment, error) {
	var p model.Payment
	err := r.db.GetContext(ctx, &p, `
		SELECT
			id, order_id, payment_id, gateway_payment_id, booking_ref, user_id, schedule_id,
			seats, amount, currency, status, payment_method, gateway,
			gateway_signature, refund_id, refund_amount, refund_status,
			refunded_at, error_code, error_description, last_error_code, expires_at, created_at, updated_at
		FROM payments
		WHERE schedule_id = $1
		  AND seats = $2
		  AND status = 'pending'
		ORDER BY created_at DESC
		LIMIT 1
	`, scheduleID, pq.Array(seats))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.AmountPaise = p.Amount
	p.RefundAmountPaise = p.RefundAmount
	return &p, nil
}

func (r *PostgresRepository) ExpirePaymentOrder(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE payments
		SET status = 'expired', updated_at = NOW()
		WHERE id = $1 AND status = 'pending'
	`, id)
	return err
}

func (r *PostgresRepository) GetPaymentByOrderID(ctx context.Context, orderID string) (*model.Payment, error) {
	var p model.Payment
	err := r.db.GetContext(ctx, &p, `
		SELECT
			id, order_id, payment_id, gateway_payment_id, booking_ref, user_id, schedule_id,
			seats, amount, currency, status, payment_method, gateway,
			gateway_signature, refund_id, refund_amount, refund_status,
			refunded_at, error_code, error_description, last_error_code, expires_at, created_at, updated_at
		FROM payments
		WHERE order_id = $1
	`, orderID)
	if err != nil {
		return nil, err
	}
	p.AmountPaise = p.Amount
	p.RefundAmountPaise = p.RefundAmount
	return &p, nil
}

func (r *PostgresRepository) GetPaymentByBookingRef(ctx context.Context, bookingRef string) (*model.Payment, error) {
	var p model.Payment
	err := r.db.GetContext(ctx, &p, `
		SELECT
			id, order_id, payment_id, gateway_payment_id, booking_ref, user_id, schedule_id,
			seats, amount, currency, status, payment_method, gateway,
			gateway_signature, refund_id, refund_amount, refund_status,
			refunded_at, error_code, error_description, last_error_code, expires_at, created_at, updated_at
		FROM payments
		WHERE booking_ref = $1
	`, bookingRef)
	if err != nil {
		return nil, err
	}
	p.AmountPaise = p.Amount
	p.RefundAmountPaise = p.RefundAmount
	return &p, nil
}

func (r *PostgresRepository) RecordPaymentFailure(ctx context.Context, orderID, paymentID, userID, errorCode, errorDesc string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE payments
		SET
			payment_id = COALESCE(NULLIF($1, ''), payment_id),
			status = 'failed',
			error_code = $2,
			error_description = $3,
			updated_at = NOW()
		WHERE order_id = $4
		  AND user_id = $5
		  AND status = 'pending'
	`, paymentID, errorCode, errorDesc, orderID, userID)
	return err
}

func (r *PostgresRepository) RecordPaymentLastError(ctx context.Context, orderID, errorCode string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE payments
		SET last_error_code = $1, updated_at = NOW()
		WHERE order_id = $2
	`, errorCode, orderID)
	return err
}

func (r *PostgresRepository) ExpireStalePendingOrders(ctx context.Context) (int64, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var acquired bool
	const advisoryLockID = 987654322
	if err := tx.GetContext(ctx, &acquired, "SELECT pg_try_advisory_xact_lock($1)", advisoryLockID); err != nil {
		return 0, err
	}
	if !acquired {
		return 0, nil
	}

	res, err := tx.ExecContext(ctx, `
		UPDATE payments
		SET status = 'expired', updated_at = NOW()
		WHERE status = 'pending'
		  AND expires_at IS NOT NULL
		  AND expires_at < NOW()
	`)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *PostgresRepository) ProcessRefund(ctx context.Context, bookingRef, refundID string, refundAmount int64) error {
	_, err := r.db.ExecContext(ctx, `
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
	return err
}

func (r *PostgresRepository) ConfirmCapturedPaymentInTx(
	ctx context.Context,
	tx *sqlx.Tx,
	in model.ConfirmPaymentInput,
) (*model.ConfirmationResult, error) {
	// 1. Lock payment row FOR UPDATE
	var payment struct {
		ID                string         `db:"id"`
		OrderID           string         `db:"order_id"`
		Status            string         `db:"status"`
		Amount            int64          `db:"amount"`
		Currency          string         `db:"currency"`
		ScheduleID        string         `db:"schedule_id"`
		UserID            string         `db:"user_id"`
		Seats             pq.StringArray `db:"seats"`
		BookingRef        *string        `db:"booking_ref"`
		PaymentID         *string        `db:"payment_id"`
		GatewayPaymentID  *string        `db:"gateway_payment_id"`
		ExpiresAt         *time.Time     `db:"expires_at"`
		TicketEmailSentAt *time.Time     `db:"ticket_email_sent_at"`
	}

	err := tx.GetContext(ctx, &payment, `
		SELECT
			id, order_id, status, amount, currency, schedule_id, user_id,
			seats, booking_ref, payment_id, gateway_payment_id, expires_at, ticket_email_sent_at
		FROM payments
		WHERE order_id = $1
		FOR UPDATE
	`, in.OrderID)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, httpx.NotFound("payment not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query payment row for update: %w", err)
	}

	if in.UserID != "" && payment.UserID != in.UserID {
		return nil, httpx.NotFound("payment not found")
	}

	paymentModel := model.Payment{
		ID:                payment.ID,
		OrderID:           payment.OrderID,
		PaymentID:         payment.PaymentID,
		GatewayPaymentID:  payment.GatewayPaymentID,
		BookingRef:        payment.BookingRef,
		UserID:            payment.UserID,
		ScheduleID:        payment.ScheduleID,
		Seats:             payment.Seats,
		Amount:            payment.Amount,
		AmountPaise:       payment.Amount,
		Currency:          payment.Currency,
		Status:            payment.Status,
		ExpiresAt:         payment.ExpiresAt,
		TicketEmailSentAt: payment.TicketEmailSentAt,
	}

	// 2. Idempotency: If already captured, return AlreadyConfirmed(existingBooking)
	if payment.Status == "captured" && payment.BookingRef != nil && *payment.BookingRef != "" {
		capturedID := ""
		if payment.GatewayPaymentID != nil && *payment.GatewayPaymentID != "" {
			capturedID = *payment.GatewayPaymentID
		} else if payment.PaymentID != nil && *payment.PaymentID != "" {
			capturedID = *payment.PaymentID
		}
		if capturedID != "" && in.PaymentID != "" && capturedID != in.PaymentID {
			return nil, httpx.BadRequest("payment order has already been captured with a different payment ID")
		}

		var existingBookings []struct {
			BookingRef      string    `db:"booking_ref"`
			ScheduleID      string    `db:"schedule_id"`
			SeatNumber      string    `db:"seat_number"`
			Status          string    `db:"status"`
			Amount          int64     `db:"amount"`
			PassengerName   string    `db:"passenger_name"`
			PassengerAge    *int      `db:"passenger_age"`
			PassengerGender *string   `db:"passenger_gender"`
			BoardingPoint   string    `db:"boarding_point"`
			DroppingPoint   string    `db:"dropping_point"`
			ContactPhone    string    `db:"contact_phone"`
			ContactEmail    string    `db:"contact_email"`
			ConfirmedAt     time.Time `db:"confirmed_at"`
		}
		_ = tx.SelectContext(ctx, &existingBookings, `
			SELECT
				booking_ref, schedule_id, seat_number, status, amount,
				passenger_name, passenger_age, passenger_gender,
				boarding_point, dropping_point, contact_phone, contact_email, confirmed_at
			FROM bookings
			WHERE booking_ref = $1
		`, *payment.BookingRef)

		var passengers []model.PassengerDetail
		boarding := in.BoardingPoint
		dropping := in.DroppingPoint
		cPhone := in.ContactPhone
		cEmail := in.ContactEmail
		confirmedAtStr := time.Now().Format(time.RFC3339)

		for _, eb := range existingBookings {
			ageVal := 0
			if eb.PassengerAge != nil {
				ageVal = *eb.PassengerAge
			}
			genderVal := ""
			if eb.PassengerGender != nil {
				genderVal = *eb.PassengerGender
			}
			passengers = append(passengers, model.PassengerDetail{
				SeatNumber: eb.SeatNumber,
				Name:       eb.PassengerName,
				Age:        ageVal,
				Gender:     genderVal,
			})
			if boarding == "" {
				boarding = eb.BoardingPoint
			}
			if dropping == "" {
				dropping = eb.DroppingPoint
			}
			if cPhone == "" {
				cPhone = eb.ContactPhone
			}
			if cEmail == "" {
				cEmail = eb.ContactEmail
			}
			confirmedAtStr = eb.ConfirmedAt.Format(time.RFC3339)
		}

		paymentMethod := in.PaymentMethod
		if paymentMethod == "" {
			paymentMethod = "card"
		}

		bookingResp := &model.VerifyPaymentResponse{
			BookingRef:       *payment.BookingRef,
			PaymentID:        in.PaymentID,
			OrderID:          in.OrderID,
			Status:           "confirmed",
			TotalAmount:      payment.Amount,
			TotalAmountPaise: payment.Amount,
			Currency:         payment.Currency,
			PaymentMethod:    paymentMethod,
			ScheduleID:       payment.ScheduleID,
			Seats:            payment.Seats,
			Passengers:       passengers,
			BoardingPoint:    boarding,
			DroppingPoint:    dropping,
			ContactPhone:     cPhone,
			ContactEmail:     cEmail,
			ConfirmedAt:      confirmedAtStr,
		}

		return &model.ConfirmationResult{
			Status:  model.ConfirmationStatusAlreadyConfirmed,
			Booking: bookingResp,
			Payment: &paymentModel,
		}, nil
	}

	// 3. If already marked refund_required
	if payment.Status == "refund_required" {
		return &model.ConfirmationResult{
			Status:            model.ConfirmationStatusNotBookable,
			NotBookableReason: "paid but not bookable: seats are unavailable, refund required",
			Payment:           &paymentModel,
		}, nil
	}

	if payment.Status != "pending" && payment.Status != "expired" {
		return nil, fmt.Errorf("payment order is already %s", payment.Status)
	}

	// Grace rule: If payment order has expired or seat locks lapsed, but the customer's payment was successfully captured,
	// we attempt to re-acquire the stored seats if they remain unbooked in seat_inventory. If all seats are still free,
	// we confirm the booking.
	var seatRows []struct {
		SeatNumber  string     `db:"seat_number"`
		Status      string     `db:"status"`
		LockedBy    *string    `db:"locked_by"`
		LockExpires *time.Time `db:"lock_expires"`
	}

	err = tx.SelectContext(ctx, &seatRows, `
		SELECT seat_number, status, locked_by, lock_expires
		FROM seat_inventory
		WHERE schedule_id = $1
		  AND seat_number = ANY($2)
		FOR UPDATE
	`, payment.ScheduleID, pq.Array(payment.Seats))
	if err != nil {
		return nil, fmt.Errorf("failed to query seat inventory for update: %w", err)
	}

	now := time.Now()
	canBook := len(seatRows) == len(payment.Seats)
	if canBook {
		for _, sr := range seatRows {
			// Seat is booked -> TAKEN!
			if sr.Status == "booked" {
				canBook = false
				break
			}
			// Seat is locked by another user and lock has not expired -> TAKEN!
			if sr.Status == "locked" && sr.LockedBy != nil && *sr.LockedBy != payment.UserID {
				if sr.LockExpires == nil || sr.LockExpires.After(now) {
					canBook = false
					break
				}
			}
		}
	}

	// If any seat is taken/gone -> mark order refund_required!
	if !canBook {
		paymentMethod := strings.TrimSpace(in.PaymentMethod)
		if paymentMethod == "" {
			paymentMethod = "card"
		}
		payID := in.PaymentID
		if payID == "" {
			payID = in.GatewayPaymentID
		}

		_, err = tx.ExecContext(ctx, `
			UPDATE payments
			SET
				status = 'refund_required',
				gateway_payment_id = COALESCE(NULLIF($1, ''), gateway_payment_id),
				payment_id = COALESCE(NULLIF($1, ''), payment_id),
				payment_method = COALESCE(NULLIF($2, ''), payment_method),
				last_error_code = 'SEAT_UNAVAILABLE_REFUND_REQUIRED',
				updated_at = NOW()
			WHERE order_id = $3
		`, payID, paymentMethod, in.OrderID)
		if err != nil {
			return nil, fmt.Errorf("failed to update payment to refund_required: %w", err)
		}

		log.Printf("[ERROR] paid but not bookable: order_id=%s, gateway_payment_id=%s", in.OrderID, payID)

		paymentModel.Status = "refund_required"
		if payID != "" {
			paymentModel.GatewayPaymentID = &payID
			paymentModel.PaymentID = &payID
		}

		return &model.ConfirmationResult{
			Status:            model.ConfirmationStatusNotBookable,
			NotBookableReason: "paid but not bookable: seats are unavailable, refund required",
			Payment:           &paymentModel,
		}, nil
	}

	// All seats are free! Proceed to book STORED seats and STORED schedule
	var farePerSeat int64
	_ = tx.GetContext(ctx, &farePerSeat, "SELECT base_fare FROM schedules WHERE id = $1", payment.ScheduleID)
	if farePerSeat <= 0 && len(payment.Seats) > 0 {
		farePerSeat = payment.Amount / int64(len(payment.Seats))
	}

	var userFallback struct {
		Name  string `db:"name"`
		Email string `db:"email"`
		Phone string `db:"phone"`
	}
	_ = tx.GetContext(ctx, &userFallback, "SELECT COALESCE(name, 'Passenger') as name, COALESCE(email, '') as email, COALESCE(phone, '') as phone FROM users WHERE id = $1", payment.UserID)

	passengerMap := make(map[string]model.PassengerDetail)
	for _, p := range in.Passengers {
		sNum := strings.ToUpper(strings.TrimSpace(p.SeatNumber))
		if sNum != "" {
			passengerMap[sNum] = p
		}
	}

	bookingRef, err := generateBookingRef()
	if err != nil {
		return nil, fmt.Errorf("failed to generate booking ref: %w", err)
	}

	boardingPoint := strings.TrimSpace(in.BoardingPoint)
	if boardingPoint == "" {
		boardingPoint = "Main Terminal"
	}
	droppingPoint := strings.TrimSpace(in.DroppingPoint)
	if droppingPoint == "" {
		droppingPoint = "Destination Terminal"
	}
	contactPhone := strings.TrimSpace(in.ContactPhone)
	if contactPhone == "" {
		contactPhone = userFallback.Phone
	}
	contactEmail := strings.TrimSpace(in.ContactEmail)
	if contactEmail == "" {
		contactEmail = userFallback.Email
	}

	var confirmedPassengers []model.PassengerDetail
	for _, seat := range payment.Seats {
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

		_, err = tx.ExecContext(ctx, `
			INSERT INTO bookings (
				booking_ref, user_id, schedule_id, seat_number, status, amount,
				passenger_name, passenger_age, passenger_gender,
				boarding_point, dropping_point, contact_phone, contact_email,
				confirmed_at, created_at
			)
			VALUES ($1, $2, $3, $4, 'confirmed', $5, $6, $7, $8, $9, $10, $11, $12, NOW(), NOW())
		`, bookingRef, payment.UserID, payment.ScheduleID, seat, farePerSeat, pName, pAge, pGender, boardingPoint, droppingPoint, contactPhone, contactEmail)
		if err != nil {
			return nil, fmt.Errorf("failed to insert booking: %w", err)
		}

		_, err = tx.ExecContext(ctx, `
			UPDATE seat_inventory
			SET
				status = 'booked',
				locked_by = NULL,
				locked_at = NULL,
				lock_expires = NULL
			WHERE schedule_id = $1
			  AND seat_number = $2
		`, payment.ScheduleID, seat)
		if err != nil {
			return nil, fmt.Errorf("failed to update seat inventory: %w", err)
		}
	}

	pMethod := strings.TrimSpace(in.PaymentMethod)
	if pMethod == "" {
		pMethod = "card"
	}
	payID := in.PaymentID
	if payID == "" {
		payID = in.GatewayPaymentID
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE payments
		SET
			payment_id = $1,
			gateway_payment_id = $1,
			booking_ref = $2,
			status = 'captured',
			payment_method = $3,
			gateway_signature = COALESCE(NULLIF($4, ''), gateway_signature),
			last_error_code = NULL,
			updated_at = NOW()
		WHERE order_id = $5
	`, payID, bookingRef, pMethod, in.Signature, in.OrderID)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return nil, httpx.BadRequest("payment ID has already been used for another booking")
		}
		return nil, fmt.Errorf("failed to update payment record: %w", err)
	}

	paymentModel.Status = "captured"
	paymentModel.BookingRef = &bookingRef
	paymentModel.PaymentID = &payID
	paymentModel.GatewayPaymentID = &payID

	bookingResp := &model.VerifyPaymentResponse{
		BookingRef:       bookingRef,
		PaymentID:        payID,
		OrderID:          in.OrderID,
		Status:           "confirmed",
		TotalAmount:      payment.Amount,
		TotalAmountPaise: payment.Amount,
		Currency:         payment.Currency,
		PaymentMethod:    pMethod,
		ScheduleID:       payment.ScheduleID,
		Seats:            payment.Seats,
		Passengers:       confirmedPassengers,
		BoardingPoint:    boardingPoint,
		DroppingPoint:    droppingPoint,
		ContactPhone:     contactPhone,
		ContactEmail:     contactEmail,
		ConfirmedAt:      time.Now().Format(time.RFC3339),
	}

	return &model.ConfirmationResult{
		Status:  model.ConfirmationStatusConfirmed,
		Booking: bookingResp,
		Payment: &paymentModel,
	}, nil
}

func (r *PostgresRepository) ConfirmCapturedPayment(
	ctx context.Context,
	in model.ConfirmPaymentInput,
) (*model.ConfirmationResult, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	res, err := r.ConfirmCapturedPaymentInTx(ctx, tx, in)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return res, nil
}

func (r *PostgresRepository) VerifyAndCapture(
	ctx context.Context,
	userID string,
	req model.VerifyPaymentRequest,
	farePerSeat int64,
) (*model.VerifyPaymentResponse, error) {
	in := model.ConfirmPaymentInput{
		OrderID:          req.OrderID,
		UserID:           userID,
		PaymentID:        req.PaymentID,
		GatewayPaymentID: req.PaymentID,
		PaymentMethod:    req.PaymentMethod,
		Signature:        req.Signature,
		Passengers:       req.Passengers,
		BoardingPoint:    req.BoardingPoint,
		DroppingPoint:    req.DroppingPoint,
		ContactPhone:     req.ContactPhone,
		ContactEmail:     req.ContactEmail,
	}

	res, err := r.ConfirmCapturedPayment(ctx, in)
	if err != nil {
		return nil, err
	}

	if res.Status == model.ConfirmationStatusNotBookable {
		var amt int64
		var curr, sched string
		var seats []string
		if res.Payment != nil {
			amt = res.Payment.Amount
			curr = res.Payment.Currency
			sched = res.Payment.ScheduleID
			seats = res.Payment.Seats
		}
		return &model.VerifyPaymentResponse{
			OrderID:          req.OrderID,
			PaymentID:        req.PaymentID,
			Status:           "refund_required",
			RefundRequired:   true,
			Message:          "payment received, booking could not be completed, refund will be processed",
			TotalAmount:      amt,
			TotalAmountPaise: amt,
			Currency:         curr,
			ScheduleID:       sched,
			Seats:            seats,
		}, nil
	}

	return res.Booking, nil
}

func (r *PostgresRepository) BeginTx(ctx context.Context) (*sqlx.Tx, error) {
	return r.db.BeginTxx(ctx, nil)
}

func (r *PostgresRepository) RecordPaymentLastErrorInTx(ctx context.Context, tx *sqlx.Tx, orderID, errorCode string) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE payments
		SET last_error_code = $1, updated_at = NOW()
		WHERE order_id = $2
	`, errorCode, orderID)
	return err
}

func (r *PostgresRepository) ClaimTicketEmailDispatch(ctx context.Context, orderID string) (bool, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE payments
		SET ticket_email_sent_at = NOW()
		WHERE order_id = $1
		  AND ticket_email_sent_at IS NULL
	`, orderID)
	if err != nil {
		return false, err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (r *PostgresRepository) RecordPaymentEventInTx(ctx context.Context, tx *sqlx.Tx, eventID, eventType, orderID, paymentID, status, errorCode string) (bool, error) {
	var insertedID int64
	err := tx.QueryRowContext(ctx, `
		INSERT INTO payment_events (event_id, event_type, order_id, payment_id, status, error_code, received_at)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5, NULLIF($6, ''), NOW())
		ON CONFLICT (event_id) DO NOTHING
		RETURNING id
	`, eventID, eventType, orderID, paymentID, status, errorCode).Scan(&insertedID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *PostgresRepository) UpdatePaymentEventInTx(ctx context.Context, tx *sqlx.Tx, eventID, status, errorCode, orderID, paymentID string) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE payment_events
		SET status = $1,
		    error_code = NULLIF($2, ''),
		    order_id = COALESCE(NULLIF($3, ''), order_id),
		    payment_id = COALESCE(NULLIF($4, ''), payment_id),
		    processed_at = NOW()
		WHERE event_id = $5
	`, status, errorCode, orderID, paymentID, eventID)
	return err
}

func (r *PostgresRepository) GetPaymentEventByEventID(ctx context.Context, eventID string) (*model.PaymentEvent, error) {
	var ev model.PaymentEvent
	err := r.db.GetContext(ctx, &ev, `
		SELECT id, event_id, event_type, order_id, payment_id, status, error_code, received_at, processed_at
		FROM payment_events
		WHERE event_id = $1
	`, eventID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ev, nil
}

func (r *PostgresRepository) ListRefundRequiredPayments(ctx context.Context) ([]model.Payment, error) {
	var payments []model.Payment
	err := r.db.SelectContext(ctx, &payments, `
		SELECT
			id, order_id, payment_id, gateway_payment_id, user_id, schedule_id,
			seats, amount, currency, status, last_error_code, created_at, updated_at
		FROM view_refund_required_payments
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	for i := range payments {
		payments[i].AmountPaise = payments[i].Amount
	}
	return payments, nil
}
