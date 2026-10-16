package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

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

func (r *PostgresRepository) GetScheduleFare(ctx context.Context, scheduleID string) (float64, string, string, error) {
	var info struct {
		BaseFare    float64 `db:"base_fare"`
		Origin      string  `db:"origin"`
		Destination string  `db:"destination"`
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
			created_at,
			updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW()
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
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
}

func (r *PostgresRepository) GetPaymentByOrderID(ctx context.Context, orderID string) (*model.Payment, error) {
	var p model.Payment
	err := r.db.GetContext(ctx, &p, `
		SELECT
			id, order_id, payment_id, booking_ref, user_id, schedule_id,
			seats, amount, currency, status, payment_method, gateway,
			gateway_signature, refund_id, refund_amount, refund_status,
			refunded_at, error_code, error_description, created_at, updated_at
		FROM payments
		WHERE order_id = $1
	`, orderID)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PostgresRepository) GetPaymentByBookingRef(ctx context.Context, bookingRef string) (*model.Payment, error) {
	var p model.Payment
	err := r.db.GetContext(ctx, &p, `
		SELECT
			id, order_id, payment_id, booking_ref, user_id, schedule_id,
			seats, amount, currency, status, payment_method, gateway,
			gateway_signature, refund_id, refund_amount, refund_status,
			refunded_at, error_code, error_description, created_at, updated_at
		FROM payments
		WHERE booking_ref = $1
	`, bookingRef)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PostgresRepository) RecordPaymentFailure(ctx context.Context, orderID, paymentID, errorCode, errorDesc string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE payments
		SET
			payment_id = COALESCE(NULLIF($1, ''), payment_id),
			status = 'failed',
			error_code = $2,
			error_description = $3,
			updated_at = NOW()
		WHERE order_id = $4
	`, paymentID, errorCode, errorDesc, orderID)
	return err
}

func (r *PostgresRepository) ProcessRefund(ctx context.Context, bookingRef, refundID string, refundAmount float64) error {
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

func (r *PostgresRepository) VerifyAndCapture(
	ctx context.Context,
	userID string,
	req model.VerifyPaymentRequest,
	farePerSeat float64,
) (*model.VerifyPaymentResponse, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 1. Lock & check payment order row
	var payment struct {
		ID         string         `db:"id"`
		Status     string         `db:"status"`
		Amount     float64        `db:"amount"`
		Currency   string         `db:"currency"`
		ScheduleID string         `db:"schedule_id"`
		UserID     string         `db:"user_id"`
		Seats      pq.StringArray `db:"seats"`
		BookingRef *string        `db:"booking_ref"`
	}

	err = tx.GetContext(ctx, &payment, `
		SELECT id, status, amount, currency, schedule_id, user_id, seats, booking_ref
		FROM payments
		WHERE order_id = $1
		FOR UPDATE
	`, req.OrderID)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("payment order not found")
	}
	if err != nil {
		return nil, err
	}

	// Idempotency check: if already captured, return the existing booking reference
	if payment.Status == "captured" && payment.BookingRef != nil && *payment.BookingRef != "" {
		return &model.VerifyPaymentResponse{
			BookingRef:    *payment.BookingRef,
			PaymentID:     req.PaymentID,
			OrderID:       req.OrderID,
			Status:        "confirmed",
			TotalAmount:   payment.Amount,
			Currency:      payment.Currency,
			PaymentMethod: req.PaymentMethod,
			ScheduleID:    payment.ScheduleID,
			Seats:         payment.Seats,
			BoardingPoint: req.BoardingPoint,
			DroppingPoint: req.DroppingPoint,
			ContactPhone:  req.ContactPhone,
			ContactEmail:  req.ContactEmail,
			ConfirmedAt:   time.Now().Format(time.RFC3339),
		}, nil
	}

	if payment.Status != "pending" {
		return nil, fmt.Errorf("payment order is already %s", payment.Status)
	}

	if payment.UserID != userID {
		return nil, errors.New("unauthorized: payment order does not belong to user")
	}

	// 2. Validate all seats locked by user
	for _, seat := range req.Seats {
		var lockInfo struct {
			Status    string `db:"status"`
			IsExpired bool   `db:"is_expired"`
		}

		err = tx.GetContext(ctx, &lockInfo, `
			SELECT
				status,
				(lock_expires IS NULL OR lock_expires < NOW()) AS is_expired
			FROM seat_inventory
			WHERE schedule_id = $1
			  AND seat_number = $2
			  AND locked_by = $3
			FOR UPDATE
		`, req.ScheduleID, seat, userID)

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

	// 3. User fallback info
	var userFallback struct {
		Name  string `db:"name"`
		Email string `db:"email"`
		Phone string `db:"phone"`
	}
	_ = tx.GetContext(ctx, &userFallback, "SELECT COALESCE(name, 'Passenger') as name, COALESCE(email, '') as email, COALESCE(phone, '') as phone FROM users WHERE id = $1", userID)

	passengerMap := make(map[string]model.PassengerDetail)
	for _, p := range req.Passengers {
		sNum := strings.ToUpper(strings.TrimSpace(p.SeatNumber))
		if sNum != "" {
			passengerMap[sNum] = p
		}
	}

	// 4. Generate unique Booking Reference
	var bookingRef string
	for attempt := 0; attempt < 10; attempt++ {
		candidate, err := generateBookingRef()
		if err != nil {
			return nil, fmt.Errorf("failed to generate booking ref: %w", err)
		}
		var exists bool
		err = tx.GetContext(ctx, &exists, "SELECT EXISTS(SELECT 1 FROM bookings WHERE booking_ref = $1)", candidate)
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

	boardingPoint := strings.TrimSpace(req.BoardingPoint)
	if boardingPoint == "" {
		boardingPoint = "Main Terminal"
	}
	droppingPoint := strings.TrimSpace(req.DroppingPoint)
	if droppingPoint == "" {
		droppingPoint = "Destination Terminal"
	}

	contactPhone := strings.TrimSpace(req.ContactPhone)
	if contactPhone == "" {
		contactPhone = userFallback.Phone
	}
	contactEmail := strings.TrimSpace(req.ContactEmail)
	if contactEmail == "" {
		contactEmail = userFallback.Email
	}

	var confirmedPassengers []model.PassengerDetail

	// 5. Insert bookings & update seat_inventory to booked
	for _, seat := range req.Seats {
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
				contact_email,
				confirmed_at,
				created_at
			)
			VALUES ($1, $2, $3, $4, 'confirmed', $5, $6, $7, $8, $9, $10, $11, $12, NOW(), NOW())
		`, bookingRef, userID, req.ScheduleID, seat, farePerSeat, pName, pAge, pGender, boardingPoint, droppingPoint, contactPhone, contactEmail)
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
		`, req.ScheduleID, seat)
		if err != nil {
			return nil, fmt.Errorf("failed to update seat inventory: %w", err)
		}
	}

	// 6. Update payment record to captured
	pMethod := strings.TrimSpace(req.PaymentMethod)
	if pMethod == "" {
		pMethod = "card"
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE payments
		SET
			payment_id = $1,
			booking_ref = $2,
			status = 'captured',
			payment_method = $3,
			gateway_signature = $4,
			updated_at = NOW()
		WHERE order_id = $5
	`, req.PaymentID, bookingRef, pMethod, req.Signature, req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("failed to update payment record: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &model.VerifyPaymentResponse{
		BookingRef:    bookingRef,
		PaymentID:     req.PaymentID,
		OrderID:       req.OrderID,
		Status:        "confirmed",
		TotalAmount:   payment.Amount,
		Currency:      payment.Currency,
		PaymentMethod: pMethod,
		ScheduleID:    req.ScheduleID,
		Seats:         req.Seats,
		Passengers:    confirmedPassengers,
		BoardingPoint: boardingPoint,
		DroppingPoint: droppingPoint,
		ContactPhone:  contactPhone,
		ContactEmail:  contactEmail,
		ConfirmedAt:   time.Now().Format(time.RFC3339),
	}, nil
}
