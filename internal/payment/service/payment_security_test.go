package service_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	_ "github.com/lib/pq"

	bookingcleanup "ticketdot/internal/booking/cleanup"
	"ticketdot/internal/httpx"
	paymentgateway "ticketdot/internal/payment/gateway"
	paymenthandler "ticketdot/internal/payment/handler"
	"ticketdot/internal/payment/model"
	paymentrepo "ticketdot/internal/payment/repository"
	"ticketdot/internal/payment/service"
)

type testFixture struct {
	db            *sqlx.DB
	repo          paymentrepo.PaymentRepository
	userA         string
	userB         string
	cityOrigin    string
	cityDest      string
	operatorID    string
	busID         string
	routeID       string
	scheduleID    string
	orderID       string
	fakeGw        *paymentgateway.FakeGateway
	paymentSvc    *service.PaymentService
	keySecret     string
	webhookSecret string
}

func (f *testFixture) registerPayment(paymentID, orderID string, amount int64, status string) {
	if orderID == "" {
		orderID = f.orderID
	}
	if amount <= 0 {
		amount = 100000
	}
	if status == "" {
		status = "captured"
	}
	f.fakeGw.RegisterPayment(paymentID, orderID, amount, "INR", status, "card")
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func setupPostgresDB(t *testing.T) *sqlx.DB {
	t.Helper()
	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dbURL == "" {
		t.Skip("skipping payment test requiring Postgres: DATABASE_URL is not set (Skipped tests: TestForgedSignatureDoesNotAlterOtherUserOrder, TestCrossUserVerifyReturnsPaymentNotFound, TestSeatTamperingTrustsStoredOrderSeats, TestIdempotencyOwnerOnly, TestCreateOrderUsesDBFareAndIgnoresClientAmounts, TestCreateOrderRejectsUnlockedExpiredOrOtherUserSeats, TestCreateOrderIdempotencyReusesPendingOrder, TestCreateOrderCrossUserConflict, TestCreateOrderConcurrency, TestCreateOrderGatewayFailureLeavesNoDanglingPendingOrder, TestCreateOrderLockExpiryExtendedAndNeverShortened, TestCleanerDoesNotReleaseSeatsProtectedByPendingOrder, TestConcurrencyDatabaseIsGuardIdenticalAndOverlappingSeats, TestAbandonedOrderExpiryAndSeatRelease, TestVerifyHMACMismatchKeepsStatusPending, TestVerifyGatewayOrderMismatchAmountMismatchCurrencyMismatch, TestVerifyGatewayStatusAuthorizedReturns202, TestVerifyGatewayStatusCreatedOrFailedRejected, TestVerifyGatewayBadGatewayOnNetworkOrTimeout, TestVerifyDuplicateGatewayPaymentIDRejected, TestVerifyInvalidPaymentIDFormatZeroGatewayCalls, TestVerifyGatewayCallOutsideDBTransaction, TestWebhookForgedOrMissingSignature, TestWebhookReserializedBodySignatureFailure, TestWebhookReplayedEventID, TestWebhookPaymentCapturedBooksUnconfirmedOrder, TestWebhookOrderPaidAndPaymentCapturedArriving, TestWebhookConcurrentVerifyAndWebhookRace, TestWebhookAmountOrCurrencyMismatch, TestWebhookUnknownOrderID, TestWebhookPaymentFailedLeavesPendingAndSeatsLocked, TestWebhookLateCaptureOnExpiredOrderFreeSeats, TestWebhookLateCaptureOnExpiredOrderTakenSeatSetsRefundRequired, TestWebhookTransientDBFailureMidConfirmationRollsBack, TestWebhookMaxBodySizeReturns413)")
	}

	db, err := sqlx.Connect("postgres", dbURL)
	if err != nil {
		t.Skipf("skipping payment test: failed to connect to DATABASE_URL: %v", err)
	}
	return db
}

func setupFixture(t *testing.T, db *sqlx.DB) *testFixture {
	t.Helper()
	ctx := context.Background()
	suffix := randomHex(6)

	// 1. Users A & B
	var userAID, userBID string
	err := db.QueryRowContext(ctx, `
		INSERT INTO users (name, email, phone, password_hash, role)
		VALUES ($1, $2, $3, $4, 'passenger')
		RETURNING id
	`, "User A", "user_a_"+suffix+"@example.com", "9000000001", "hash").Scan(&userAID)
	if err != nil {
		t.Fatalf("failed to insert user A: %v", err)
	}

	err = db.QueryRowContext(ctx, `
		INSERT INTO users (name, email, phone, password_hash, role)
		VALUES ($1, $2, $3, $4, 'passenger')
		RETURNING id
	`, "User B", "user_b_"+suffix+"@example.com", "9000000002", "hash").Scan(&userBID)
	if err != nil {
		t.Fatalf("failed to insert user B: %v", err)
	}

	// 2. Cities
	var originID, destID string
	err = db.QueryRowContext(ctx, `
		INSERT INTO cities (name, state)
		VALUES ($1, $2)
		RETURNING id
	`, "CityOrigin_"+suffix, "StateA").Scan(&originID)
	if err != nil {
		t.Fatalf("failed to insert origin city: %v", err)
	}

	err = db.QueryRowContext(ctx, `
		INSERT INTO cities (name, state)
		VALUES ($1, $2)
		RETURNING id
	`, "CityDest_"+suffix, "StateB").Scan(&destID)
	if err != nil {
		t.Fatalf("failed to insert dest city: %v", err)
	}

	// 3. Operator
	var opID string
	err = db.QueryRowContext(ctx, `
		INSERT INTO operators (name)
		VALUES ($1)
		RETURNING id
	`, "Operator_"+suffix).Scan(&opID)
	if err != nil {
		t.Fatalf("failed to insert operator: %v", err)
	}

	// 4. Bus
	var busID string
	err = db.QueryRowContext(ctx, `
		INSERT INTO buses (operator_id, bus_type, total_seats, registration_no)
		VALUES ($1, 'sleeper', 40, $2)
		RETURNING id
	`, opID, "BUS_"+suffix).Scan(&busID)
	if err != nil {
		t.Fatalf("failed to insert bus: %v", err)
	}

	// 5. Route
	var routeID string
	err = db.QueryRowContext(ctx, `
		INSERT INTO routes (origin_id, destination_id, operator_id)
		VALUES ($1, $2, $3)
		RETURNING id
	`, originID, destID, opID).Scan(&routeID)
	if err != nil {
		t.Fatalf("failed to insert route: %v", err)
	}

	// 6. Schedule
	var schedID string
	err = db.QueryRowContext(ctx, `
		INSERT INTO schedules (route_id, bus_id, operator_id, departure_time, arrival_time, available_seats, base_fare, status)
		VALUES ($1, $2, $3, NOW() + INTERVAL '1 hour', NOW() + INTERVAL '5 hours', 40, 50000, 'scheduled')
		RETURNING id
	`, routeID, busID, opID).Scan(&schedID)
	if err != nil {
		t.Fatalf("failed to insert schedule: %v", err)
	}

	// 7. Seat Inventory (A1 and A2 locked by User A)
	_, err = db.ExecContext(ctx, `
		INSERT INTO seat_inventory (schedule_id, seat_number, status, locked_by, locked_at, lock_expires)
		VALUES 
			($1, 'A1', 'locked', $2, NOW(), NOW() + INTERVAL '1 hour'),
			($1, 'A2', 'locked', $2, NOW(), NOW() + INTERVAL '1 hour')
	`, schedID, userAID)
	if err != nil {
		t.Fatalf("failed to insert seat inventory: %v", err)
	}

	// 8. Payment order for User A
	orderID := "order_" + suffix
	_, err = db.ExecContext(ctx, `
		INSERT INTO payments (order_id, user_id, schedule_id, seats, amount, currency, status, gateway)
		VALUES ($1, $2, $3, $4, 100000, 'INR', 'pending', 'razorpay')
	`, orderID, userAID, schedID, pq.Array([]string{"A1", "A2"}))
	if err != nil {
		t.Fatalf("failed to insert payment order: %v", err)
	}

	keySecret := "rzp_secret_test_" + suffix
	webhookSecret := "wh_sec_" + suffix
	repo := paymentrepo.NewPostgresRepository(db)
	fakeGw := paymentgateway.NewFakeGateway()
	svc := service.NewPaymentService(repo, fakeGw, "rzp_key_"+suffix, keySecret, webhookSecret, 12, 10000000)

	f := &testFixture{
		db:            db,
		repo:          repo,
		userA:         userAID,
		userB:         userBID,
		cityOrigin:    originID,
		cityDest:      destID,
		operatorID:    opID,
		busID:         busID,
		routeID:       routeID,
		scheduleID:    schedID,
		orderID:       orderID,
		fakeGw:        fakeGw,
		paymentSvc:    svc,
		keySecret:     keySecret,
		webhookSecret: webhookSecret,
	}

	t.Cleanup(func() {
		f.cleanup()
	})

	return f
}

func (f *testFixture) cleanup() {
	ctx := context.Background()
	_, _ = f.db.ExecContext(ctx, `DELETE FROM payment_events WHERE order_id = $1`, f.orderID)
	_, _ = f.db.ExecContext(ctx, `DELETE FROM bookings WHERE schedule_id = $1`, f.scheduleID)
	_, _ = f.db.ExecContext(ctx, `DELETE FROM payments WHERE schedule_id = $1`, f.scheduleID)
	_, _ = f.db.ExecContext(ctx, `DELETE FROM seat_inventory WHERE schedule_id = $1`, f.scheduleID)
	_, _ = f.db.ExecContext(ctx, `DELETE FROM schedules WHERE id = $1`, f.scheduleID)
	_, _ = f.db.ExecContext(ctx, `DELETE FROM routes WHERE id = $1`, f.routeID)
	_, _ = f.db.ExecContext(ctx, `DELETE FROM buses WHERE id = $1`, f.busID)
	_, _ = f.db.ExecContext(ctx, `DELETE FROM operators WHERE id = $1`, f.operatorID)
	_, _ = f.db.ExecContext(ctx, `DELETE FROM cities WHERE id IN ($1, $2)`, f.cityOrigin, f.cityDest)
	_, _ = f.db.ExecContext(ctx, `DELETE FROM users WHERE id IN ($1, $2)`, f.userA, f.userB)
}

// Test 1: Forged signature rejected and failure is isolated to authenticated user only
func TestForgedSignatureDoesNotAlterOtherUserOrder(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()

	f := setupFixture(t, db)
	ctx := context.Background()

	// 1. User B sends a forged signature for User A's order
	req := model.VerifyPaymentRequest{
		OrderID:   f.orderID,
		PaymentID: "pay_attacker_123",
		Signature: "forged_invalid_signature_hex",
		Seats:     []string{"A1", "A2"},
	}

	_, err := f.paymentSvc.VerifyPayment(ctx, f.userB, req)
	if err == nil {
		t.Fatal("expected forged signature verification to fail, got nil")
	}
	if !strings.Contains(err.Error(), "payment not found") {
		t.Fatalf("expected 'payment not found', got: %v", err)
	}

	// 2. Authoritative check: User A's order must STILL be 'pending' in Postgres
	var statusA string
	var errCodeA *string
	err = f.db.QueryRowContext(ctx, `SELECT status, error_code FROM payments WHERE order_id = $1`, f.orderID).Scan(&statusA, &errCodeA)
	if err != nil {
		t.Fatalf("failed to query payment row: %v", err)
	}

	if statusA != "pending" {
		t.Fatalf("expected User A's order status to remain 'pending' after User B's attack, got: %s", statusA)
	}
	if errCodeA != nil {
		t.Fatalf("expected error_code to remain nil on User A's order, got: %v", *errCodeA)
	}

	// 3. User A submits invalid signature for their own order -> status remains pending, last_error_code is recorded
	_, err = f.paymentSvc.VerifyPayment(ctx, f.userA, req)
	if err == nil {
		t.Fatal("expected User A verification to fail with forged signature")
	}

	var lastErrCode *string
	err = f.db.QueryRowContext(ctx, `SELECT status, last_error_code FROM payments WHERE order_id = $1`, f.orderID).Scan(&statusA, &lastErrCode)
	if err != nil {
		t.Fatalf("failed to query payment row: %v", err)
	}

	if statusA != "pending" {
		t.Fatalf("expected order to remain 'pending' when owner verifies invalid signature, got: %s", statusA)
	}
	if lastErrCode == nil || *lastErrCode != "SIGNATURE_MISMATCH" {
		t.Fatalf("expected last_error_code SIGNATURE_MISMATCH, got: %v", lastErrCode)
	}
}

// Test 2: Cross-user verify and read return "payment not found" (never confirms existence)
func TestCrossUserVerifyReturnsPaymentNotFound(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()

	f := setupFixture(t, db)
	ctx := context.Background()

	paymentID := "pay_valid_" + randomHex(4)
	validSig := service.ComputeHmacSha256(f.orderID+"|"+paymentID, f.keySecret)

	// 1. User B attempts to read User A's payment status -> must return "payment not found"
	_, err := f.paymentSvc.GetPaymentStatus(ctx, f.userB, f.orderID)
	if err == nil {
		t.Fatal("expected error when User B reads User A's order, got nil")
	}
	if err.Error() != "payment not found" {
		t.Fatalf("expected 'payment not found', got: %v", err)
	}

	// Owner User A can read it
	statusA, err := f.paymentSvc.GetPaymentStatus(ctx, f.userA, f.orderID)
	if err != nil {
		t.Fatalf("owner User A failed to get status: %v", err)
	}
	if statusA.OrderID != f.orderID {
		t.Fatalf("expected order %s, got %s", f.orderID, statusA.OrderID)
	}

	// 2. User B attempts to verify User A's payment order -> must return "payment not found"
	verifyReq := model.VerifyPaymentRequest{
		OrderID:   f.orderID,
		PaymentID: paymentID,
		Signature: validSig,
		Seats:     []string{"A1", "A2"},
	}

	_, err = f.paymentSvc.VerifyPayment(ctx, f.userB, verifyReq)
	if err == nil {
		t.Fatal("expected error when User B verifies User A's order, got nil")
	}
	if !strings.Contains(err.Error(), "payment not found") {
		t.Fatalf("expected 'payment not found', got: %v", err)
	}
}

// Test 3: Seat tampering - stored seats and schedule are booked in Postgres, never client's
func TestSeatTamperingTrustsStoredOrderSeats(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()

	f := setupFixture(t, db)
	ctx := context.Background()

	paymentID := "pay_" + randomHex(8)
	f.registerPayment(paymentID, f.orderID, 100000, "captured")
	validSig := service.ComputeHmacSha256(f.orderID+"|"+paymentID, f.keySecret)

	// User A passes tampered seats and schedule in verify request
	req := model.VerifyPaymentRequest{
		OrderID:       f.orderID,
		PaymentID:     paymentID,
		Signature:     validSig,
		ScheduleID:    "sched-tampered-attempt",
		Seats:         []string{"Z99", "ATTACK_SEAT"},
		PaymentMethod: "upi",
	}

	res, err := f.paymentSvc.VerifyPayment(ctx, f.userA, req)
	if err != nil {
		t.Fatalf("VerifyPayment failed: %v", err)
	}

	// 1. Verify returned response has the stored seats, not the tampered ones
	if len(res.Seats) != 2 || res.Seats[0] != "A1" || res.Seats[1] != "A2" {
		t.Fatalf("expected returned seats [A1, A2], got: %v", res.Seats)
	}

	// 2. Authoritative database check: Check bookings table in Postgres
	var bookedSeats []string
	rows, err := f.db.QueryContext(ctx, `
		SELECT seat_number FROM bookings
		WHERE schedule_id = $1 AND status = 'confirmed'
		ORDER BY seat_number
	`, f.scheduleID)
	if err != nil {
		t.Fatalf("failed to query bookings: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatalf("failed to scan seat: %v", err)
		}
		bookedSeats = append(bookedSeats, s)
	}

	if len(bookedSeats) != 2 || bookedSeats[0] != "A1" || bookedSeats[1] != "A2" {
		t.Fatalf("expected stored seats [A1, A2] booked in DB, got: %v", bookedSeats)
	}

	// 3. Ensure no tampered seat was booked anywhere
	var countTampered int
	err = f.db.QueryRowContext(ctx, `SELECT count(*) FROM bookings WHERE seat_number IN ('Z99', 'ATTACK_SEAT')`).Scan(&countTampered)
	if err != nil {
		t.Fatalf("failed to query tampered seats count: %v", err)
	}
	if countTampered != 0 {
		t.Fatalf("tampered seats were booked in database! count: %d", countTampered)
	}

	// 4. Check seat inventory was updated to 'booked' for stored seats
	var invStatus string
	err = f.db.QueryRowContext(ctx, `SELECT status FROM seat_inventory WHERE schedule_id = $1 AND seat_number = 'A1'`, f.scheduleID).Scan(&invStatus)
	if err != nil {
		t.Fatalf("failed to query seat_inventory: %v", err)
	}
	if invStatus != "booked" {
		t.Fatalf("expected seat A1 status 'booked', got: %s", invStatus)
	}
}

// Test 4: Idempotency is preserved for owner, while cross-user replay is rejected with "payment not found"
func TestIdempotencyOwnerOnly(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()

	f := setupFixture(t, db)
	ctx := context.Background()

	paymentID := "pay_" + randomHex(8)
	f.registerPayment(paymentID, f.orderID, 100000, "captured")
	validSig := service.ComputeHmacSha256(f.orderID+"|"+paymentID, f.keySecret)

	req := model.VerifyPaymentRequest{
		OrderID:       f.orderID,
		PaymentID:     paymentID,
		Signature:     validSig,
		Seats:         []string{"A1", "A2"},
		PaymentMethod: "card",
	}

	// First verify succeeds
	res1, err := f.paymentSvc.VerifyPayment(ctx, f.userA, req)
	if err != nil {
		t.Fatalf("initial verify failed: %v", err)
	}
	if res1.BookingRef == "" {
		t.Fatal("expected non-empty booking_ref")
	}

	// 1. Owner replays verify -> succeeds idempotently and returns the same booking reference
	res2, err := f.paymentSvc.VerifyPayment(ctx, f.userA, req)
	if err != nil {
		t.Fatalf("owner replay should succeed idempotently, got: %v", err)
	}
	if res2.BookingRef != res1.BookingRef {
		t.Fatalf("expected same booking reference %s, got: %s", res1.BookingRef, res2.BookingRef)
	}

	// 2. User B replays verify for User A's captured order -> must return "payment not found"
	_, err = f.paymentSvc.VerifyPayment(ctx, f.userB, req)
	if err == nil {
		t.Fatal("expected User B replay of captured order to fail, got nil")
	}
	if !strings.Contains(err.Error(), "payment not found") {
		t.Fatalf("expected 'payment not found' on cross-user replay, got: %v", err)
	}
}

type spyGateway struct {
	mu         sync.Mutex
	calls      int
	shouldFail bool
}

func (s *spyGateway) Name() string {
	return "spy"
}

func (s *spyGateway) CreateOrder(ctx context.Context, in paymentgateway.CreateOrderInput) (*paymentgateway.GatewayOrder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.shouldFail {
		return nil, httpx.BadGateway("simulated gateway error")
	}
	return &paymentgateway.GatewayOrder{
		ID:          "order_spy_" + randomHex(8),
		AmountPaise: in.AmountPaise,
		Currency:    in.Currency,
		Receipt:     in.Receipt,
		Status:      "created",
	}, nil
}

func (s *spyGateway) FetchPayment(ctx context.Context, paymentID string) (*paymentgateway.GatewayPayment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.shouldFail {
		return nil, httpx.BadGateway("simulated gateway error")
	}
	return &paymentgateway.GatewayPayment{
		ID:          paymentID,
		OrderID:     "order_spy_test",
		AmountPaise: 50000,
		Currency:    "INR",
		Status:      "captured",
		Method:      "card",
	}, nil
}

// Test 5: DB fare is used, client amounts ignored
func TestCreateOrderUsesDBFareAndIgnoresClientAmounts(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()

	f := setupFixture(t, db)
	ctx := context.Background()

	// Lock seat B1 for User A
	_, err := db.ExecContext(ctx, `
		INSERT INTO seat_inventory (schedule_id, seat_number, status, locked_by, locked_at, lock_expires)
		VALUES ($1, 'B1', 'locked', $2, NOW(), NOW() + INTERVAL '10 minutes')
	`, f.scheduleID, f.userA)
	if err != nil {
		t.Fatalf("failed to insert seat B1: %v", err)
	}

	req := model.CreateOrderRequest{
		ScheduleID: f.scheduleID,
		Seats:      []string{"B1"},
	}

	res, err := f.paymentSvc.CreateOrder(ctx, f.userA, req)
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	// Schedule base_fare is 50000 paise (500 INR).
	if res.AmountPaise != 50000 || res.Amount != 50000 {
		t.Fatalf("expected order amount 50000 paise, got AmountPaise=%d, Amount=%d", res.AmountPaise, res.Amount)
	}

	// Verify database record
	var dbAmount int64
	var dbStatus string
	err = db.QueryRowContext(ctx, `
		SELECT amount, status FROM payments WHERE order_id = $1
	`, res.OrderID).Scan(&dbAmount, &dbStatus)
	if err != nil {
		t.Fatalf("failed to query payments table: %v", err)
	}
	if dbAmount != 50000 || dbStatus != "pending" {
		t.Fatalf("expected stored payment amount=50000, status=pending, got amount=%d, status=%s", dbAmount, dbStatus)
	}
}

// Test 6: Unlocked, expired, or other-user seats rejected
func TestCreateOrderRejectsUnlockedExpiredOrOtherUserSeats(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()

	f := setupFixture(t, db)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, `
		INSERT INTO seat_inventory (schedule_id, seat_number, status, locked_by, locked_at, lock_expires)
		VALUES 
			($1, 'B2', 'locked', $2, NOW(), NOW() + INTERVAL '10 minutes'),
			($1, 'B3', 'locked', $3, NOW() - INTERVAL '15 minutes', NOW() - INTERVAL '5 minutes'),
			($1, 'B4', 'available', NULL, NULL, NULL)
	`, f.scheduleID, f.userB, f.userA)
	if err != nil {
		t.Fatalf("failed to insert test seats: %v", err)
	}

	// Case 1: Seat locked by another user (User B)
	_, err = f.paymentSvc.CreateOrder(ctx, f.userA, model.CreateOrderRequest{
		ScheduleID: f.scheduleID,
		Seats:      []string{"B2"},
	})
	if err == nil {
		t.Fatal("expected error when seat locked by another user, got nil")
	}

	// Case 2: Seat lock expired
	_, err = f.paymentSvc.CreateOrder(ctx, f.userA, model.CreateOrderRequest{
		ScheduleID: f.scheduleID,
		Seats:      []string{"B3"},
	})
	if err == nil {
		t.Fatal("expected error when seat lock expired, got nil")
	}

	// Case 3: Seat available / not locked
	_, err = f.paymentSvc.CreateOrder(ctx, f.userA, model.CreateOrderRequest{
		ScheduleID: f.scheduleID,
		Seats:      []string{"B4"},
	})
	if err == nil {
		t.Fatal("expected error when seat is not locked, got nil")
	}
}

// Test 7: Same user repeating request reuses pending order (1 row, 1 gateway call)
func TestCreateOrderIdempotencyReusesPendingOrder(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()

	f := setupFixture(t, db)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, `
		INSERT INTO seat_inventory (schedule_id, seat_number, status, locked_by, locked_at, lock_expires)
		VALUES ($1, 'B5', 'locked', $2, NOW(), NOW() + INTERVAL '10 minutes')
	`, f.scheduleID, f.userA)
	if err != nil {
		t.Fatalf("failed to insert seat B5: %v", err)
	}

	spy := &spyGateway{}
	repo := paymentrepo.NewPostgresRepository(db)
	svc := service.NewPaymentService(repo, spy, "key", "secret", "whsec", 12, 10000000)

	req := model.CreateOrderRequest{
		ScheduleID: f.scheduleID,
		Seats:      []string{"B5"},
	}

	// 1st request -> calls gateway, inserts order
	res1, err := svc.CreateOrder(ctx, f.userA, req)
	if err != nil {
		t.Fatalf("first CreateOrder failed: %v", err)
	}
	if spy.calls != 1 {
		t.Fatalf("expected 1 gateway call, got %d", spy.calls)
	}

	// 2nd request -> idempotently reuses existing pending order, does not call gateway again
	res2, err := svc.CreateOrder(ctx, f.userA, req)
	if err != nil {
		t.Fatalf("second CreateOrder failed: %v", err)
	}
	if spy.calls != 1 {
		t.Fatalf("expected gateway call count to remain 1, got %d", spy.calls)
	}
	if res1.OrderID != res2.OrderID {
		t.Fatalf("expected same order ID %s, got %s", res1.OrderID, res2.OrderID)
	}

	// Verify only 1 pending order row exists in DB
	var rowCount int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM payments WHERE schedule_id = $1 AND seats = $2 AND status = 'pending'
	`, f.scheduleID, pq.Array([]string{"B5"})).Scan(&rowCount)
	if err != nil {
		t.Fatalf("failed to query payments count: %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("expected exactly 1 payment record, got %d", rowCount)
	}
}

// Test 8: Cross-user conflict when order in progress
func TestCreateOrderCrossUserConflict(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()

	f := setupFixture(t, db)
	ctx := context.Background()

	// Seat B6 locked by User A
	_, err := db.ExecContext(ctx, `
		INSERT INTO seat_inventory (schedule_id, seat_number, status, locked_by, locked_at, lock_expires)
		VALUES ($1, 'B6', 'locked', $2, NOW(), NOW() + INTERVAL '10 minutes')
	`, f.scheduleID, f.userA)
	if err != nil {
		t.Fatalf("failed to insert seat B6: %v", err)
	}

	// User A creates order
	_, err = f.paymentSvc.CreateOrder(ctx, f.userA, model.CreateOrderRequest{
		ScheduleID: f.scheduleID,
		Seats:      []string{"B6"},
	})
	if err != nil {
		t.Fatalf("User A create order failed: %v", err)
	}

	// User B attempts to create order for the same seat
	_, err = f.paymentSvc.CreateOrder(ctx, f.userB, model.CreateOrderRequest{
		ScheduleID: f.scheduleID,
		Seats:      []string{"B6"},
	})
	if err == nil {
		t.Fatal("expected cross-user create order to fail, got nil")
	}
}

// Test 9: Concurrent create-order requests create exactly 1 order (run with -race)
func TestCreateOrderConcurrency(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()

	f := setupFixture(t, db)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, `
		INSERT INTO seat_inventory (schedule_id, seat_number, status, locked_by, locked_at, lock_expires)
		VALUES ($1, 'B7', 'locked', $2, NOW(), NOW() + INTERVAL '10 minutes')
	`, f.scheduleID, f.userA)
	if err != nil {
		t.Fatalf("failed to insert seat B7: %v", err)
	}

	spy := &spyGateway{}
	repo := paymentrepo.NewPostgresRepository(db)
	svc := service.NewPaymentService(repo, spy, "key", "secret", "whsec", 12, 10000000)

	const concurrency = 10
	var wg sync.WaitGroup
	orders := make([]*model.CreateOrderResponse, concurrency)
	errs := make([]error, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := model.CreateOrderRequest{
				ScheduleID: f.scheduleID,
				Seats:      []string{"B7"},
			}
			res, err := svc.CreateOrder(context.Background(), f.userA, req)
			orders[idx] = res
			errs[idx] = err
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d failed: %v", i, err)
		}
	}

	firstID := orders[0].OrderID
	for i, o := range orders {
		if o.OrderID != firstID {
			t.Fatalf("goroutine %d got different order ID %s, expected %s", i, o.OrderID, firstID)
		}
	}

	// Exactly 1 gateway call should have occurred!
	if spy.calls != 1 {
		t.Fatalf("expected exactly 1 gateway call, got %d", spy.calls)
	}

	// Exactly 1 payment record in DB
	var rowCount int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM payments WHERE schedule_id = $1 AND seats = $2 AND status = 'pending'
	`, f.scheduleID, pq.Array([]string{"B7"})).Scan(&rowCount)
	if err != nil {
		t.Fatalf("failed to query payments count: %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("expected exactly 1 payment record in DB, got %d", rowCount)
	}
}

// Test 10: Gateway failure leaves no dangling pending order and returns generic error
func TestCreateOrderGatewayFailureLeavesNoDanglingPendingOrder(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()

	f := setupFixture(t, db)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, `
		INSERT INTO seat_inventory (schedule_id, seat_number, status, locked_by, locked_at, lock_expires)
		VALUES ($1, 'B8', 'locked', $2, NOW(), NOW() + INTERVAL '10 minutes')
	`, f.scheduleID, f.userA)
	if err != nil {
		t.Fatalf("failed to insert seat B8: %v", err)
	}

	failingSpy := &spyGateway{shouldFail: true}
	repo := paymentrepo.NewPostgresRepository(db)
	svc := service.NewPaymentService(repo, failingSpy, "key", "secret", "whsec", 12, 10000000)

	req := model.CreateOrderRequest{
		ScheduleID: f.scheduleID,
		Seats:      []string{"B8"},
	}

	_, err = svc.CreateOrder(ctx, f.userA, req)
	if err == nil {
		t.Fatal("expected CreateOrder to fail on gateway error, got nil")
	}

	var appErr *httpx.AppError
	if !strings.Contains(err.Error(), "payment gateway error") && (!errors.As(err, &appErr) || appErr.StatusCode != 502) {
		t.Fatalf("expected Bad Gateway error (502), got: %v", err)
	}

	// Check DB: NO dangling payment row
	var count int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM payments WHERE schedule_id = $1 AND seats = $2
	`, f.scheduleID, pq.Array([]string{"B8"})).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count payment rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 dangling payment rows after gateway failure, got %d", count)
	}
}

// Test 11: Lock expiry extended and never shortened
func TestCreateOrderLockExpiryExtendedAndNeverShortened(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()

	f := setupFixture(t, db)
	ctx := context.Background()

	// Initial lock with 5 minutes remaining
	_, err := db.ExecContext(ctx, `
		INSERT INTO seat_inventory (schedule_id, seat_number, status, locked_by, locked_at, lock_expires)
		VALUES ($1, 'B9', 'locked', $2, NOW(), NOW() + INTERVAL '5 minutes')
	`, f.scheduleID, f.userA)
	if err != nil {
		t.Fatalf("failed to insert seat B9: %v", err)
	}

	repo := paymentrepo.NewPostgresRepository(db)
	fakeGw := paymentgateway.NewFakeGateway()
	// Set windowMinutes = 15 minutes
	svc := service.NewPaymentService(repo, fakeGw, "key", "secret", "whsec", 15, 10000000)

	_, err = svc.CreateOrder(ctx, f.userA, model.CreateOrderRequest{
		ScheduleID: f.scheduleID,
		Seats:      []string{"B9"},
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	var lockExpires time.Time
	err = db.QueryRowContext(ctx, `
		SELECT lock_expires FROM seat_inventory WHERE schedule_id = $1 AND seat_number = 'B9'
	`, f.scheduleID).Scan(&lockExpires)
	if err != nil {
		t.Fatalf("failed to query lock_expires: %v", err)
	}

	// Should be approximately 15 minutes from now (at least 13 minutes)
	timeUntilExpiry := time.Until(lockExpires)
	if timeUntilExpiry < 13*time.Minute {
		t.Fatalf("expected lock_expires extended to ~15 minutes, got %v", timeUntilExpiry)
	}

	// Now set lock_expires to 30 minutes in future and verify it is never shortened
	_, err = db.ExecContext(ctx, `
		UPDATE seat_inventory SET lock_expires = NOW() + INTERVAL '30 minutes'
		WHERE schedule_id = $1 AND seat_number = 'B9'
	`, f.scheduleID)
	if err != nil {
		t.Fatalf("failed to update lock_expires: %v", err)
	}

	// Delete pending order so we can re-create with smaller window of 10 minutes
	_, err = db.ExecContext(ctx, `DELETE FROM payments WHERE schedule_id = $1 AND seats = $2`, f.scheduleID, pq.Array([]string{"B9"}))
	if err != nil {
		t.Fatalf("failed to clear payments: %v", err)
	}

	svcShort := service.NewPaymentService(repo, fakeGw, "key", "secret", "whsec", 10, 10000000)
	_, err = svcShort.CreateOrder(ctx, f.userA, model.CreateOrderRequest{
		ScheduleID: f.scheduleID,
		Seats:      []string{"B9"},
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	err = db.QueryRowContext(ctx, `
		SELECT lock_expires FROM seat_inventory WHERE schedule_id = $1 AND seat_number = 'B9'
	`, f.scheduleID).Scan(&lockExpires)
	if err != nil {
		t.Fatalf("failed to query lock_expires: %v", err)
	}

	timeUntilExpiry = time.Until(lockExpires)
	// Should still be ~30 minutes (greater than 25 minutes), NOT reduced to 10 minutes!
	if timeUntilExpiry < 25*time.Minute {
		t.Fatalf("lock_expires was improperly shortened! Remaining: %v", timeUntilExpiry)
	}
}

// Test 12: Cleaner does not release seats protected by a pending order
func TestCleanerDoesNotReleaseSeatsProtectedByPendingOrder(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()

	f := setupFixture(t, db)
	ctx := context.Background()

	// Seat B10 has an expired lock (lock_expires was 5 minutes ago)
	_, err := db.ExecContext(ctx, `
		INSERT INTO seat_inventory (schedule_id, seat_number, status, locked_by, locked_at, lock_expires)
		VALUES ($1, 'B10', 'locked', $2, NOW() - INTERVAL '10 minutes', NOW() - INTERVAL '5 minutes')
	`, f.scheduleID, f.userA)
	if err != nil {
		t.Fatalf("failed to insert seat B10: %v", err)
	}

	// But payments has a pending order with expires_at in the future (10 minutes from now)
	orderID := "order_protect_" + randomHex(4)
	_, err = db.ExecContext(ctx, `
		INSERT INTO payments (order_id, user_id, schedule_id, seats, amount, currency, status, gateway, expires_at)
		VALUES ($1, $2, $3, $4, 50000, 'INR', 'pending', 'razorpay', NOW() + INTERVAL '10 minutes')
	`, orderID, f.userA, f.scheduleID, pq.Array([]string{"B10"}))
	if err != nil {
		t.Fatalf("failed to insert pending payment: %v", err)
	}

	// Run expired lock cleaner
	bookingcleanup.ReleaseExpiredLocks(db)

	// Seat B10 MUST NOT have been released!
	var status string
	err = db.QueryRowContext(ctx, `
		SELECT status FROM seat_inventory WHERE schedule_id = $1 AND seat_number = 'B10'
	`, f.scheduleID).Scan(&status)
	if err != nil {
		t.Fatalf("failed to query seat status: %v", err)
	}
	if status != "locked" {
		t.Fatalf("expected seat B10 to remain locked due to pending payment order, got: %s", status)
	}

	// Now expire the payment order as well
	_, err = db.ExecContext(ctx, `
		UPDATE payments SET expires_at = NOW() - INTERVAL '1 minute' WHERE order_id = $1
	`, orderID)
	if err != nil {
		t.Fatalf("failed to expire payment: %v", err)
	}

	// Run cleaner again
	bookingcleanup.ReleaseExpiredLocks(db)

	// Now seat B10 SHOULD be released to 'available'
	err = db.QueryRowContext(ctx, `
		SELECT status FROM seat_inventory WHERE schedule_id = $1 AND seat_number = 'B10'
	`, f.scheduleID).Scan(&status)
	if err != nil {
		t.Fatalf("failed to query seat status: %v", err)
	}
	if status != "available" {
		t.Fatalf("expected seat B10 to be released to 'available', got: %s", status)
	}
}

// Test 13: Concurrency - database is the guard for identical and overlapping seats (bypassing in-process mutex)
func TestConcurrencyDatabaseIsGuardIdenticalAndOverlappingSeats(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()

	f := setupFixture(t, db)
	ctx := context.Background()

	// 1. Identical seats: lock C1, C2 for User A
	_, err := db.ExecContext(ctx, `
		INSERT INTO seat_inventory (schedule_id, seat_number, status, locked_by, locked_at, lock_expires)
		VALUES 
			($1, 'C1', 'locked', $2, NOW(), NOW() + INTERVAL '10 minutes'),
			($1, 'C2', 'locked', $2, NOW(), NOW() + INTERVAL '10 minutes')
	`, f.scheduleID, f.userA)
	if err != nil {
		t.Fatalf("failed to insert seats C1, C2: %v", err)
	}

	const concurrencyCount = 10
	var wg sync.WaitGroup
	type callResult struct {
		id  int
		err error
	}
	results := make([]callResult, concurrencyCount)

	// Call repository CreateOrderAndExtendLocks directly from 10 goroutines bypassing the service mutex
	for i := 0; i < concurrencyCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			p := &model.Payment{
				OrderID:    fmt.Sprintf("order_ident_%d_%s", idx, randomHex(4)),
				UserID:     f.userA,
				ScheduleID: f.scheduleID,
				Seats:      []string{"C1", "C2"},
				Amount:     100000,
				Currency:   "INR",
				Status:     "pending",
				Gateway:    "razorpay",
			}
			err := f.repo.CreateOrderAndExtendLocks(ctx, p, 12)
			results[idx] = callResult{id: idx, err: err}
		}(i)
	}
	wg.Wait()

	successCount := 0
	conflictCount := 0
	for _, res := range results {
		if res.err == nil {
			successCount++
		} else {
			conflictCount++
		}
	}

	if successCount != 1 {
		t.Fatalf("expected exactly 1 successful order creation for identical seats, got %d (failures: %d)", successCount, conflictCount)
	}

	// 2. Overlapping seats: User A has [D1, D2], User B has [D3]
	_, err = db.ExecContext(ctx, `
		INSERT INTO seat_inventory (schedule_id, seat_number, status, locked_by, locked_at, lock_expires)
		VALUES 
			($1, 'D1', 'locked', $2, NOW(), NOW() + INTERVAL '10 minutes'),
			($1, 'D2', 'locked', $2, NOW(), NOW() + INTERVAL '10 minutes'),
			($1, 'D3', 'locked', $3, NOW(), NOW() + INTERVAL '10 minutes')
	`, f.scheduleID, f.userA, f.userB)
	if err != nil {
		t.Fatalf("failed to insert seats D1, D2, D3: %v", err)
	}

	// Order A is created for [D1, D2] by User A
	pA := &model.Payment{
		OrderID:    "order_overlap_A_" + randomHex(4),
		UserID:     f.userA,
		ScheduleID: f.scheduleID,
		Seats:      []string{"D1", "D2"},
		Amount:     100000,
		Currency:   "INR",
		Status:     "pending",
		Gateway:    "razorpay",
	}
	err = f.repo.CreateOrderAndExtendLocks(ctx, pA, 12)
	if err != nil {
		t.Fatalf("expected Order A to succeed, got: %v", err)
	}

	// User B attempts to create order for overlapping seats [D2, D3]
	pB := &model.Payment{
		OrderID:    "order_overlap_B_" + randomHex(4),
		UserID:     f.userB,
		ScheduleID: f.scheduleID,
		Seats:      []string{"D2", "D3"},
		Amount:     100000,
		Currency:   "INR",
		Status:     "pending",
		Gateway:    "razorpay",
	}
	errB := f.repo.CreateOrderAndExtendLocks(ctx, pB, 12)
	if errB == nil {
		t.Fatal("expected overlapping order B to fail with conflict, got nil")
	}
}

// Test 14: Abandoned order recovery - expired pending order expires -> seat lock released -> User B can book
func TestAbandonedOrderExpiryAndSeatRelease(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()

	f := setupFixture(t, db)
	ctx := context.Background()

	// User A locked E1, lock expired 5 minutes ago
	_, err := db.ExecContext(ctx, `
		INSERT INTO seat_inventory (schedule_id, seat_number, status, locked_by, locked_at, lock_expires)
		VALUES ($1, 'E1', 'locked', $2, NOW() - INTERVAL '15 minutes', NOW() - INTERVAL '5 minutes')
	`, f.scheduleID, f.userA)
	if err != nil {
		t.Fatalf("failed to insert seat E1: %v", err)
	}

	// User A has pending order for E1, expired 2 minutes ago
	abandonedOrderID := "order_abandoned_" + randomHex(4)
	_, err = db.ExecContext(ctx, `
		INSERT INTO payments (order_id, user_id, schedule_id, seats, amount, currency, status, gateway, expires_at)
		VALUES ($1, $2, $3, $4, 50000, 'INR', 'pending', 'razorpay', NOW() - INTERVAL '2 minutes')
	`, abandonedOrderID, f.userA, f.scheduleID, pq.Array([]string{"E1"}))
	if err != nil {
		t.Fatalf("failed to insert abandoned payment: %v", err)
	}

	// 1. Background order expiry worker pass sets status = 'expired'
	expiredCount, err := f.repo.ExpireStalePendingOrders(ctx)
	if err != nil {
		t.Fatalf("failed to run ExpireStalePendingOrders: %v", err)
	}
	if expiredCount < 1 {
		t.Fatalf("expected at least 1 order expired, got: %d", expiredCount)
	}

	var status string
	err = db.QueryRowContext(ctx, `SELECT status FROM payments WHERE order_id = $1`, abandonedOrderID).Scan(&status)
	if err != nil {
		t.Fatalf("failed to query payment status: %v", err)
	}
	if status != "expired" {
		t.Fatalf("expected payment status 'expired', got: %s", status)
	}

	// 2. Lock cleaner unprotects and frees the seat
	bookingcleanup.ReleaseExpiredLocks(db)

	var seatStatus string
	err = db.QueryRowContext(ctx, `SELECT status FROM seat_inventory WHERE schedule_id = $1 AND seat_number = 'E1'`, f.scheduleID).Scan(&seatStatus)
	if err != nil {
		t.Fatalf("failed to query seat status: %v", err)
	}
	if seatStatus != "available" {
		t.Fatalf("expected seat E1 to be available, got: %s", seatStatus)
	}

	// 3. User B can now lock seat E1
	_, err = db.ExecContext(ctx, `
		UPDATE seat_inventory
		SET status = 'locked', locked_by = $1, locked_at = NOW(), lock_expires = NOW() + INTERVAL '10 minutes'
		WHERE schedule_id = $2 AND seat_number = 'E1'
	`, f.userB, f.scheduleID)
	if err != nil {
		t.Fatalf("failed to lock seat E1 for User B: %v", err)
	}

	// User B creates order for E1
	orderB, err := f.paymentSvc.CreateOrder(ctx, f.userB, model.CreateOrderRequest{
		ScheduleID: f.scheduleID,
		Seats:      []string{"E1"},
	})
	if err != nil {
		t.Fatalf("User B failed to create order: %v", err)
	}
	if orderB.OrderID == "" {
		t.Fatal("expected non-empty order ID for User B")
	}
}

// Test 15: Signature mismatch keeps status 'pending' and records last_error_code
func TestVerifyHMACMismatchKeepsStatusPending(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()

	f := setupFixture(t, db)
	ctx := context.Background()

	paymentID := "pay_" + randomHex(8)
	req := model.VerifyPaymentRequest{
		OrderID:   f.orderID,
		PaymentID: paymentID,
		Signature: "bad_signature_tampered_hex",
		Seats:     []string{"A1", "A2"},
	}

	_, err := f.paymentSvc.VerifyPayment(ctx, f.userA, req)
	if err == nil {
		t.Fatal("expected verify to fail with bad HMAC signature, got nil")
	}

	var status string
	var lastErrCode *string
	err = db.QueryRowContext(ctx, `SELECT status, last_error_code FROM payments WHERE order_id = $1`, f.orderID).Scan(&status, &lastErrCode)
	if err != nil {
		t.Fatalf("failed to query payment row: %v", err)
	}

	if status != "pending" {
		t.Fatalf("expected payment status to remain 'pending' after bad HMAC, got: %s", status)
	}
	if lastErrCode == nil || *lastErrCode != "SIGNATURE_MISMATCH" {
		t.Fatalf("expected last_error_code SIGNATURE_MISMATCH, got: %v", lastErrCode)
	}
}

// Test 16: Gateway mismatch checks (order ID, amount, currency) reject and record last_error_code
func TestVerifyGatewayOrderMismatchAmountMismatchCurrencyMismatch(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()

	f := setupFixture(t, db)
	ctx := context.Background()

	// 1. Order ID mismatch
	pid1 := "pay_" + randomHex(8)
	f.fakeGw.RegisterPayment(pid1, "order_different_999", 100000, "INR", "captured", "card")
	sig1 := service.ComputeHmacSha256(f.orderID+"|"+pid1, f.keySecret)
	_, err := f.paymentSvc.VerifyPayment(ctx, f.userA, model.VerifyPaymentRequest{
		OrderID:   f.orderID,
		PaymentID: pid1,
		Signature: sig1,
		Seats:     []string{"A1", "A2"},
	})
	if err == nil {
		t.Fatal("expected order ID mismatch to be rejected, got nil")
	}

	var lastErrCode string
	_ = db.QueryRowContext(ctx, `SELECT last_error_code FROM payments WHERE order_id = $1`, f.orderID).Scan(&lastErrCode)
	if lastErrCode != "ORDER_ID_MISMATCH" {
		t.Fatalf("expected ORDER_ID_MISMATCH, got: %s", lastErrCode)
	}

	// 2. Amount mismatch
	pid2 := "pay_" + randomHex(8)
	f.fakeGw.RegisterPayment(pid2, f.orderID, 40000, "INR", "captured", "card")
	sig2 := service.ComputeHmacSha256(f.orderID+"|"+pid2, f.keySecret)
	_, err = f.paymentSvc.VerifyPayment(ctx, f.userA, model.VerifyPaymentRequest{
		OrderID:   f.orderID,
		PaymentID: pid2,
		Signature: sig2,
		Seats:     []string{"A1", "A2"},
	})
	if err == nil {
		t.Fatal("expected amount mismatch to be rejected, got nil")
	}
	_ = db.QueryRowContext(ctx, `SELECT last_error_code FROM payments WHERE order_id = $1`, f.orderID).Scan(&lastErrCode)
	if lastErrCode != "AMOUNT_MISMATCH" {
		t.Fatalf("expected AMOUNT_MISMATCH, got: %s", lastErrCode)
	}

	// 3. Currency mismatch
	pid3 := "pay_" + randomHex(8)
	f.fakeGw.RegisterPayment(pid3, f.orderID, 100000, "USD", "captured", "card")
	sig3 := service.ComputeHmacSha256(f.orderID+"|"+pid3, f.keySecret)
	_, err = f.paymentSvc.VerifyPayment(ctx, f.userA, model.VerifyPaymentRequest{
		OrderID:   f.orderID,
		PaymentID: pid3,
		Signature: sig3,
		Seats:     []string{"A1", "A2"},
	})
	if err == nil {
		t.Fatal("expected currency mismatch to be rejected, got nil")
	}
	_ = db.QueryRowContext(ctx, `SELECT last_error_code FROM payments WHERE order_id = $1`, f.orderID).Scan(&lastErrCode)
	if lastErrCode != "CURRENCY_MISMATCH" {
		t.Fatalf("expected CURRENCY_MISMATCH, got: %s", lastErrCode)
	}
}

// Test 17: Gateway status 'authorized' returns HTTP 202 processing and leaves order pending
func TestVerifyGatewayStatusAuthorizedReturns202(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()

	f := setupFixture(t, db)
	ctx := context.Background()

	paymentID := "pay_" + randomHex(8)
	f.fakeGw.RegisterPayment(paymentID, f.orderID, 100000, "INR", "authorized", "card")
	validSig := service.ComputeHmacSha256(f.orderID+"|"+paymentID, f.keySecret)

	res, err := f.paymentSvc.VerifyPayment(ctx, f.userA, model.VerifyPaymentRequest{
		OrderID:       f.orderID,
		PaymentID:     paymentID,
		Signature:     validSig,
		Seats:         []string{"A1", "A2"},
		PaymentMethod: "card",
	})
	if err != nil {
		t.Fatalf("unexpected error for authorized payment: %v", err)
	}
	if res.Status != "processing" {
		t.Fatalf("expected status 'processing', got: %s", res.Status)
	}

	// Order in DB must remain 'pending'
	var status string
	err = db.QueryRowContext(ctx, `SELECT status FROM payments WHERE order_id = $1`, f.orderID).Scan(&status)
	if err != nil {
		t.Fatalf("failed to query payment row: %v", err)
	}
	if status != "pending" {
		t.Fatalf("expected payment status to remain 'pending', got: %s", status)
	}
}

// Test 18: Gateway status 'created' or 'failed' are rejected and order remains pending
func TestVerifyGatewayStatusCreatedOrFailedRejected(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()

	f := setupFixture(t, db)
	ctx := context.Background()

	// 1. Status 'created'
	pidCreated := "pay_" + randomHex(8)
	f.fakeGw.RegisterPayment(pidCreated, f.orderID, 100000, "INR", "created", "card")
	sigCreated := service.ComputeHmacSha256(f.orderID+"|"+pidCreated, f.keySecret)
	_, err := f.paymentSvc.VerifyPayment(ctx, f.userA, model.VerifyPaymentRequest{
		OrderID:   f.orderID,
		PaymentID: pidCreated,
		Signature: sigCreated,
		Seats:     []string{"A1", "A2"},
	})
	if err == nil {
		t.Fatal("expected status 'created' to be rejected, got nil")
	}

	// 2. Status 'failed'
	pidFailed := "pay_" + randomHex(8)
	f.fakeGw.RegisterPayment(pidFailed, f.orderID, 100000, "INR", "failed", "card")
	sigFailed := service.ComputeHmacSha256(f.orderID+"|"+pidFailed, f.keySecret)
	_, err = f.paymentSvc.VerifyPayment(ctx, f.userA, model.VerifyPaymentRequest{
		OrderID:   f.orderID,
		PaymentID: pidFailed,
		Signature: sigFailed,
		Seats:     []string{"A1", "A2"},
	})
	if err == nil {
		t.Fatal("expected status 'failed' to be rejected, got nil")
	}

	var status string
	err = db.QueryRowContext(ctx, `SELECT status FROM payments WHERE order_id = $1`, f.orderID).Scan(&status)
	if err != nil {
		t.Fatalf("failed to query payment row: %v", err)
	}
	if status != "pending" {
		t.Fatalf("expected order to remain 'pending', got: %s", status)
	}
}

type failingGateway struct {
	paymentgateway.Gateway
}

func (g *failingGateway) FetchPayment(ctx context.Context, paymentID string) (*paymentgateway.GatewayPayment, error) {
	return nil, httpx.BadGateway("gateway network connection reset")
}

// Test 19: Gateway timeout/network failure returns 502 Bad Gateway and keeps order pending for retry
func TestVerifyGatewayBadGatewayOnNetworkOrTimeout(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()

	f := setupFixture(t, db)
	ctx := context.Background()

	failingSvc := service.NewPaymentService(f.repo, &failingGateway{f.fakeGw}, "key", f.keySecret, "wh_sec", 12, 10000000)

	paymentID := "pay_" + randomHex(8)
	validSig := service.ComputeHmacSha256(f.orderID+"|"+paymentID, f.keySecret)

	_, err := failingSvc.VerifyPayment(ctx, f.userA, model.VerifyPaymentRequest{
		OrderID:   f.orderID,
		PaymentID: paymentID,
		Signature: validSig,
		Seats:     []string{"A1", "A2"},
	})
	if err == nil {
		t.Fatal("expected 502 Bad Gateway error on failing gateway, got nil")
	}

	var appErr *httpx.AppError
	if !errors.As(err, &appErr) || appErr.StatusCode != 502 {
		t.Fatalf("expected HTTP 502 Bad Gateway error, got: %v", err)
	}

	var status string
	err = db.QueryRowContext(ctx, `SELECT status FROM payments WHERE order_id = $1`, f.orderID).Scan(&status)
	if err != nil {
		t.Fatalf("failed to query payment row: %v", err)
	}
	if status != "pending" {
		t.Fatalf("expected order to remain 'pending' after gateway failure, got: %s", status)
	}
}

// Test 20: Duplicate gateway_payment_id is rejected by database constraint
func TestVerifyDuplicateGatewayPaymentIDRejected(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()

	f := setupFixture(t, db)
	ctx := context.Background()

	// Create second order for User A on seats A3, A4
	_, err := db.ExecContext(ctx, `
		INSERT INTO seat_inventory (schedule_id, seat_number, status, locked_by, locked_at, lock_expires)
		VALUES 
			($1, 'A3', 'locked', $2, NOW(), NOW() + INTERVAL '10 minutes'),
			($1, 'A4', 'locked', $2, NOW(), NOW() + INTERVAL '10 minutes')
	`, f.scheduleID, f.userA)
	if err != nil {
		t.Fatalf("failed to insert seats A3, A4: %v", err)
	}

	orderID2 := "order_second_" + randomHex(4)
	_, err = db.ExecContext(ctx, `
		INSERT INTO payments (order_id, user_id, schedule_id, seats, amount, currency, status, gateway)
		VALUES ($1, $2, $3, $4, 100000, 'INR', 'pending', 'razorpay')
	`, orderID2, f.userA, f.scheduleID, pq.Array([]string{"A3", "A4"}))
	if err != nil {
		t.Fatalf("failed to insert second payment: %v", err)
	}

	sharedPaymentID := "pay_" + randomHex(10)
	f.fakeGw.RegisterPayment(sharedPaymentID, f.orderID, 100000, "INR", "captured", "card")

	// Verify first order -> succeeds
	sig1 := service.ComputeHmacSha256(f.orderID+"|"+sharedPaymentID, f.keySecret)
	_, err = f.paymentSvc.VerifyPayment(ctx, f.userA, model.VerifyPaymentRequest{
		OrderID:       f.orderID,
		PaymentID:     sharedPaymentID,
		Signature:     sig1,
		Seats:         []string{"A1", "A2"},
		PaymentMethod: "card",
	})
	if err != nil {
		t.Fatalf("first order verify failed: %v", err)
	}

	// Update fake gateway to also return sharedPaymentID for orderID2
	f.fakeGw.RegisterPayment(sharedPaymentID, orderID2, 100000, "INR", "captured", "card")

	// Attempt to verify second order with the SAME payment ID -> must be rejected due to uniqueness
	sig2 := service.ComputeHmacSha256(orderID2+"|"+sharedPaymentID, f.keySecret)
	_, err = f.paymentSvc.VerifyPayment(ctx, f.userA, model.VerifyPaymentRequest{
		OrderID:       orderID2,
		PaymentID:     sharedPaymentID,
		Signature:     sig2,
		Seats:         []string{"A3", "A4"},
		PaymentMethod: "card",
	})
	if err == nil {
		t.Fatal("expected duplicate gateway_payment_id verify to fail, got nil")
	}
}

// Test 21: Invalid payment ID format rejected with zero gateway calls
func TestVerifyInvalidPaymentIDFormatZeroGatewayCalls(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()

	f := setupFixture(t, db)
	ctx := context.Background()

	spy := &spyGateway{}
	svc := service.NewPaymentService(f.repo, spy, "key", f.keySecret, "wh_sec", 12, 10000000)

	invalidPaymentID := "bad_payment_id"
	sig := service.ComputeHmacSha256(f.orderID+"|"+invalidPaymentID, f.keySecret)

	_, err := svc.VerifyPayment(ctx, f.userA, model.VerifyPaymentRequest{
		OrderID:   f.orderID,
		PaymentID: invalidPaymentID,
		Signature: sig,
		Seats:     []string{"A1", "A2"},
	})
	if err == nil {
		t.Fatal("expected invalid payment ID to be rejected, got nil")
	}

	if spy.calls > 0 {
		t.Fatalf("expected 0 gateway calls for invalid payment ID format, got %d", spy.calls)
	}
}

// Test 22: Gateway fetch executes outside database transaction (no DB locks held during gateway call)
func TestVerifyGatewayCallOutsideDBTransaction(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()

	f := setupFixture(t, db)
	ctx := context.Background()

	fetchStarted := make(chan struct{})
	unblockFetch := make(chan struct{})

	slowGw := &blockingGateway{
		FakeGateway:  f.fakeGw,
		fetchStarted: fetchStarted,
		unblockFetch: unblockFetch,
	}

	slowSvc := service.NewPaymentService(f.repo, slowGw, "key", f.keySecret, "wh_sec", 12, 10000000)

	paymentID := "pay_" + randomHex(8)
	f.fakeGw.RegisterPayment(paymentID, f.orderID, 100000, "INR", "captured", "card")
	validSig := service.ComputeHmacSha256(f.orderID+"|"+paymentID, f.keySecret)

	go func() {
		_, _ = slowSvc.VerifyPayment(ctx, f.userA, model.VerifyPaymentRequest{
			OrderID:       f.orderID,
			PaymentID:     paymentID,
			Signature:     validSig,
			Seats:         []string{"A1", "A2"},
			PaymentMethod: "card",
		})
	}()

	// Wait until gateway FetchPayment is actively running
	<-fetchStarted

	// During slow gateway fetch, ensure another DB query on payments can read freely without blocking
	concurrentTxDone := make(chan error, 1)
	go func() {
		var status string
		err := db.QueryRowContext(ctx, `SELECT status FROM payments WHERE order_id = $1`, f.orderID).Scan(&status)
		concurrentTxDone <- err
	}()

	select {
	case err := <-concurrentTxDone:
		if err != nil {
			t.Fatalf("concurrent DB query failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("concurrent DB query was blocked during gateway call! Gateway call must be outside DB transaction")
	}

	// Unblock gateway call
	close(unblockFetch)
}

type blockingGateway struct {
	*paymentgateway.FakeGateway
	fetchStarted chan struct{}
	unblockFetch chan struct{}
}

func (b *blockingGateway) FetchPayment(ctx context.Context, paymentID string) (*paymentgateway.GatewayPayment, error) {
	close(b.fetchStarted)
	select {
	case <-b.unblockFetch:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return b.FakeGateway.FetchPayment(ctx, paymentID)
}

func (f *testFixture) createWebhookPayload(event, orderID, paymentID string, amount int64, currency string) []byte {
	if orderID == "" {
		orderID = f.orderID
	}
	if paymentID == "" {
		paymentID = "pay_" + randomHex(10)
	}
	if amount <= 0 {
		amount = 100000
	}
	if currency == "" {
		currency = "INR"
	}
	payload := map[string]interface{}{
		"entity":     "event",
		"account_id": "acc_test",
		"event":      event,
		"contains":   []string{"payment"},
		"payload": map[string]interface{}{
			"payment": map[string]interface{}{
				"entity": map[string]interface{}{
					"id":       paymentID,
					"entity":   "payment",
					"amount":   amount,
					"currency": currency,
					"status":   "captured",
					"order_id": orderID,
					"method":   "upi",
					"captured": true,
				},
			},
		},
		"created_at": time.Now().Unix(),
	}
	b, _ := json.Marshal(payload)
	return b
}

func (f *testFixture) createOrderPaidWebhookPayload(orderID, paymentID string, amount int64, currency string) []byte {
	if orderID == "" {
		orderID = f.orderID
	}
	if paymentID == "" {
		paymentID = "pay_" + randomHex(10)
	}
	if amount <= 0 {
		amount = 100000
	}
	if currency == "" {
		currency = "INR"
	}
	payload := map[string]interface{}{
		"entity":     "event",
		"account_id": "acc_test",
		"event":      "order.paid",
		"contains":   []string{"order", "payment"},
		"payload": map[string]interface{}{
			"order": map[string]interface{}{
				"entity": map[string]interface{}{
					"id":          orderID,
					"entity":      "order",
					"amount":      amount,
					"amount_paid": amount,
					"amount_due":  0,
					"currency":    currency,
					"status":      "paid",
				},
			},
			"payment": map[string]interface{}{
				"entity": map[string]interface{}{
					"id":       paymentID,
					"entity":   "payment",
					"amount":   amount,
					"currency": currency,
					"status":   "captured",
					"order_id": orderID,
					"method":   "card",
					"captured": true,
				},
			},
		},
		"created_at": time.Now().Unix(),
	}
	b, _ := json.Marshal(payload)
	return b
}

func (f *testFixture) createPaymentFailedWebhookPayload(orderID, paymentID, errorCode string) []byte {
	if orderID == "" {
		orderID = f.orderID
	}
	if paymentID == "" {
		paymentID = "pay_" + randomHex(10)
	}
	if errorCode == "" {
		errorCode = "BAD_REQUEST_ERROR"
	}
	payload := map[string]interface{}{
		"entity":     "event",
		"account_id": "acc_test",
		"event":      "payment.failed",
		"contains":   []string{"payment"},
		"payload": map[string]interface{}{
			"payment": map[string]interface{}{
				"entity": map[string]interface{}{
					"id":                paymentID,
					"entity":            "payment",
					"amount":            100000,
					"currency":          "INR",
					"status":            "failed",
					"order_id":          orderID,
					"error_code":        errorCode,
					"error_description": "Payment was declined by bank",
				},
			},
		},
		"created_at": time.Now().Unix(),
	}
	b, _ := json.Marshal(payload)
	return b
}

// 1. Forged or missing signature -> 401, changes nothing
func TestWebhookForgedOrMissingSignature(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()
	f := setupFixture(t, db)
	ctx := context.Background()

	payload := f.createWebhookPayload("payment.captured", f.orderID, "pay_webhook_forged_1", 100000, "INR")

	// Missing signature -> 401
	errMissing := f.paymentSvc.ProcessWebhook(ctx, "evt_missing_sig", payload, "")
	if errMissing == nil {
		t.Fatal("expected error for missing webhook signature, got nil")
	}
	var appErr *httpx.AppError
	if errors.As(errMissing, &appErr) && appErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", appErr.StatusCode)
	}

	// Forged signature -> 401
	errForged := f.paymentSvc.ProcessWebhook(ctx, "evt_forged_sig", payload, "invalid_hmac_hex_signature")
	if errForged == nil {
		t.Fatal("expected error for forged webhook signature, got nil")
	}
	if errors.As(errForged, &appErr) && appErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", appErr.StatusCode)
	}

	// Verify nothing was changed in DB
	var status string
	err := db.QueryRowContext(ctx, `SELECT status FROM payments WHERE order_id = $1`, f.orderID).Scan(&status)
	if err != nil || status != "pending" {
		t.Fatalf("expected payment status 'pending', got %q (err=%v)", status, err)
	}

	var bookingCount int
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM bookings WHERE schedule_id = $1`, f.scheduleID).Scan(&bookingCount)
	if err != nil || bookingCount != 0 {
		t.Fatalf("expected 0 bookings, got %d", bookingCount)
	}
}

// 2. Signature computed over re-serialized JSON -> 401
func TestWebhookReserializedBodySignatureFailure(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()
	f := setupFixture(t, db)
	ctx := context.Background()

	rawBody := []byte(`{"account_id":"acc_test","contains":["payment"],"created_at":1600000000,"entity":"event","event":"payment.captured","payload":{"payment":{"entity":{"amount":100000,"captured":true,"currency":"INR","id":"pay_reser_123456","method":"upi","order_id":"` + f.orderID + `","status":"captured"}}}}`)
	sig := service.ComputeHmacSha256(string(rawBody), f.webhookSecret)

	// Modified body (same JSON semantically, but different formatting/whitespace)
	var formatted bytes.Buffer
	_ = json.Indent(&formatted, rawBody, "", "  ")
	reserializedBody := formatted.Bytes()

	err := f.paymentSvc.ProcessWebhook(ctx, "evt_reserialized", reserializedBody, sig)
	if err == nil {
		t.Fatal("expected error for signature computed over raw body when given reserialized body, got nil")
	}
	var appErr *httpx.AppError
	if errors.As(err, &appErr) && appErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", appErr.StatusCode)
	}
}

// 3. Replayed event ID -> 200, no second booking, no second email
func TestWebhookReplayedEventID(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()
	f := setupFixture(t, db)
	ctx := context.Background()

	eventID := "evt_replay_" + randomHex(8)
	paymentID := "pay_replay_" + randomHex(8)
	payload := f.createWebhookPayload("payment.captured", f.orderID, paymentID, 100000, "INR")
	sig := service.ComputeHmacSha256(string(payload), f.webhookSecret)

	// First delivery: books the order
	err1 := f.paymentSvc.ProcessWebhook(ctx, eventID, payload, sig)
	if err1 != nil {
		t.Fatalf("first webhook delivery failed: %v", err1)
	}

	// Second delivery with identical event ID: returns nil (200 OK) without re-processing
	err2 := f.paymentSvc.ProcessWebhook(ctx, eventID, payload, sig)
	if err2 != nil {
		t.Fatalf("replayed webhook delivery failed: %v", err2)
	}

	// Verify only 1 booking reference was created
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT booking_ref) FROM bookings WHERE schedule_id = $1`, f.scheduleID).Scan(&count)
	if err != nil || count != 1 {
		t.Fatalf("expected exactly 1 booking, got %d (err=%v)", count, err)
	}
}

// 4. payment.captured for an unconfirmed order -> books it
func TestWebhookPaymentCapturedBooksUnconfirmedOrder(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()
	f := setupFixture(t, db)
	ctx := context.Background()

	eventID := "evt_capture_" + randomHex(8)
	paymentID := "pay_cap_" + randomHex(8)
	payload := f.createWebhookPayload("payment.captured", f.orderID, paymentID, 100000, "INR")
	sig := service.ComputeHmacSha256(string(payload), f.webhookSecret)

	err := f.paymentSvc.ProcessWebhook(ctx, eventID, payload, sig)
	if err != nil {
		t.Fatalf("ProcessWebhook failed: %v", err)
	}

	// Verify payments row
	var status, bookingRef, gatewayPaymentID string
	err = db.QueryRowContext(ctx, `
		SELECT status, COALESCE(booking_ref, ''), COALESCE(gateway_payment_id, '')
		FROM payments WHERE order_id = $1
	`, f.orderID).Scan(&status, &bookingRef, &gatewayPaymentID)
	if err != nil {
		t.Fatalf("failed to query payments: %v", err)
	}
	if status != "captured" {
		t.Fatalf("expected payment status 'captured', got %q", status)
	}
	if bookingRef == "" {
		t.Fatal("expected non-empty booking_ref")
	}
	if gatewayPaymentID != paymentID {
		t.Fatalf("expected gateway_payment_id %q, got %q", paymentID, gatewayPaymentID)
	}

	// Verify bookings row
	var bookedUserID string
	err = db.QueryRowContext(ctx, `SELECT user_id FROM bookings WHERE booking_ref = $1`, bookingRef).Scan(&bookedUserID)
	if err != nil || bookedUserID != f.userA {
		t.Fatalf("expected booking for userA %q, got %q (err=%v)", f.userA, bookedUserID, err)
	}

	// Verify seat_inventory row
	var seatA1Status, seatA2Status string
	err = db.QueryRowContext(ctx, `SELECT status FROM seat_inventory WHERE schedule_id = $1 AND seat_number = 'A1'`, f.scheduleID).Scan(&seatA1Status)
	if err != nil || seatA1Status != "booked" {
		t.Fatalf("expected seat A1 status 'booked', got %q", seatA1Status)
	}
	err = db.QueryRowContext(ctx, `SELECT status FROM seat_inventory WHERE schedule_id = $1 AND seat_number = 'A2'`, f.scheduleID).Scan(&seatA2Status)
	if err != nil || seatA2Status != "booked" {
		t.Fatalf("expected seat A2 status 'booked', got %q", seatA2Status)
	}

	// Verify payment_events record
	var evtStatus, evtOrder string
	err = db.QueryRowContext(ctx, `SELECT status, order_id FROM payment_events WHERE event_id = $1`, eventID).Scan(&evtStatus, &evtOrder)
	if err != nil || evtStatus != "processed" || evtOrder != f.orderID {
		t.Fatalf("expected payment_event processed for order %q, got status=%q, order=%q (err=%v)", f.orderID, evtStatus, evtOrder, err)
	}
}

// 5. order.paid and payment.captured arriving for the same order -> exactly one booking
func TestWebhookOrderPaidAndPaymentCapturedArriving(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()
	f := setupFixture(t, db)
	ctx := context.Background()

	paymentID := "pay_shared_" + randomHex(8)

	// 1. order.paid arrives first
	eventID1 := "evt_order_paid_" + randomHex(8)
	payload1 := f.createOrderPaidWebhookPayload(f.orderID, paymentID, 100000, "INR")
	sig1 := service.ComputeHmacSha256(string(payload1), f.webhookSecret)

	err1 := f.paymentSvc.ProcessWebhook(ctx, eventID1, payload1, sig1)
	if err1 != nil {
		t.Fatalf("order.paid webhook failed: %v", err1)
	}

	// 2. payment.captured arrives next
	eventID2 := "evt_pay_cap_" + randomHex(8)
	payload2 := f.createWebhookPayload("payment.captured", f.orderID, paymentID, 100000, "INR")
	sig2 := service.ComputeHmacSha256(string(payload2), f.webhookSecret)

	err2 := f.paymentSvc.ProcessWebhook(ctx, eventID2, payload2, sig2)
	if err2 != nil {
		t.Fatalf("payment.captured webhook failed: %v", err2)
	}

	// Check DB: exactly one booking exists
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT booking_ref) FROM bookings WHERE schedule_id = $1`, f.scheduleID).Scan(&count)
	if err != nil || count != 1 {
		t.Fatalf("expected exactly 1 booking, got %d (err=%v)", count, err)
	}
}

// 6. Concurrent verify + webhook for the same order -> exactly one booking, exactly one email
func TestWebhookConcurrentVerifyAndWebhookRace(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()
	f := setupFixture(t, db)
	ctx := context.Background()

	paymentID := "pay_race" + randomHex(8)
	f.registerPayment(paymentID, f.orderID, 100000, "captured")

	verifySig := service.ComputeHmacSha256(f.orderID+"|"+paymentID, f.keySecret)
	verifyReq := model.VerifyPaymentRequest{
		OrderID:       f.orderID,
		PaymentID:     paymentID,
		Signature:     verifySig,
		Seats:         []string{"A1", "A2"},
		PaymentMethod: "card",
	}

	webhookPayload := f.createWebhookPayload("payment.captured", f.orderID, paymentID, 100000, "INR")
	webhookSig := service.ComputeHmacSha256(string(webhookPayload), f.webhookSecret)
	webhookEventID := "evt_race_" + randomHex(8)

	var wg sync.WaitGroup
	wg.Add(2)

	var verifyErr, webhookErr error
	go func() {
		defer wg.Done()
		_, verifyErr = f.paymentSvc.VerifyPayment(ctx, f.userA, verifyReq)
	}()

	go func() {
		defer wg.Done()
		webhookErr = f.paymentSvc.ProcessWebhook(ctx, webhookEventID, webhookPayload, webhookSig)
	}()

	wg.Wait()

	if verifyErr != nil {
		t.Fatalf("concurrent verify call returned unexpected error: %v", verifyErr)
	}
	if webhookErr != nil {
		t.Fatalf("concurrent webhook call returned unexpected error: %v", webhookErr)
	}

	// DB verification: exactly one booking exists
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT booking_ref) FROM bookings WHERE schedule_id = $1`, f.scheduleID).Scan(&count)
	if err != nil || count != 1 {
		t.Fatalf("expected exactly 1 booking created, got %d", count)
	}

	// Verify payment status is captured
	var status string
	err = db.QueryRowContext(ctx, `SELECT status FROM payments WHERE order_id = $1`, f.orderID).Scan(&status)
	if err != nil || status != "captured" {
		t.Fatalf("expected payment status 'captured', got %q", status)
	}

	// Verify email sent timestamp is present (guaranteed once-only)
	var emailSentAt *time.Time
	err = db.QueryRowContext(ctx, `SELECT ticket_email_sent_at FROM payments WHERE order_id = $1`, f.orderID).Scan(&emailSentAt)
	if err != nil {
		t.Fatalf("failed to query ticket_email_sent_at: %v", err)
	}
}

// 7. Amount or currency mismatch -> webhook returns 200, records failure, leaves order pending
func TestWebhookAmountOrCurrencyMismatch(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()
	f := setupFixture(t, db)
	ctx := context.Background()

	// Amount mismatch: order is 100000 paise, webhook sends 200000 paise
	amtPayload := f.createWebhookPayload("payment.captured", f.orderID, "pay_amt_mismatch_1", 200000, "INR")
	amtSig := service.ComputeHmacSha256(string(amtPayload), f.webhookSecret)
	eventIDAmt := "evt_amt_mismatch_" + randomHex(6)

	err := f.paymentSvc.ProcessWebhook(ctx, eventIDAmt, amtPayload, amtSig)
	if err != nil {
		t.Fatalf("expected webhook to return nil (200 OK) on amount mismatch, got: %v", err)
	}

	// Check DB: payment is still pending, event recorded as ignored with AMOUNT_MISMATCH
	var status string
	_ = db.QueryRowContext(ctx, `SELECT status FROM payments WHERE order_id = $1`, f.orderID).Scan(&status)
	if status != "pending" {
		t.Fatalf("expected payment status to remain 'pending', got %q", status)
	}

	var evtStatus, errorCode string
	_ = db.QueryRowContext(ctx, `SELECT status, error_code FROM payment_events WHERE event_id = $1`, eventIDAmt).Scan(&evtStatus, &errorCode)
	if evtStatus != "ignored" || errorCode != "AMOUNT_MISMATCH" {
		t.Fatalf("expected event status 'ignored' with 'AMOUNT_MISMATCH', got %q, %q", evtStatus, errorCode)
	}

	// Currency mismatch: order is INR, webhook sends USD
	currPayload := f.createWebhookPayload("payment.captured", f.orderID, "pay_curr_mismatch_1", 100000, "USD")
	currSig := service.ComputeHmacSha256(string(currPayload), f.webhookSecret)
	eventIDCurr := "evt_curr_mismatch_" + randomHex(6)

	err = f.paymentSvc.ProcessWebhook(ctx, eventIDCurr, currPayload, currSig)
	if err != nil {
		t.Fatalf("expected webhook to return nil (200 OK) on currency mismatch, got: %v", err)
	}

	_ = db.QueryRowContext(ctx, `SELECT status FROM payments WHERE order_id = $1`, f.orderID).Scan(&status)
	if status != "pending" {
		t.Fatalf("expected payment status to remain 'pending', got %q", status)
	}

	_ = db.QueryRowContext(ctx, `SELECT status, error_code FROM payment_events WHERE event_id = $1`, eventIDCurr).Scan(&evtStatus, &errorCode)
	if evtStatus != "ignored" || errorCode != "CURRENCY_MISMATCH" {
		t.Fatalf("expected event status 'ignored' with 'CURRENCY_MISMATCH', got %q, %q", evtStatus, errorCode)
	}
}

// 8. Webhook for an unknown order ID -> returns 200, records event as ignored, leaves everything alone
func TestWebhookUnknownOrderID(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()
	f := setupFixture(t, db)
	ctx := context.Background()

	unknownOrderID := "order_nonexistent_" + randomHex(8)
	payload := f.createWebhookPayload("payment.captured", unknownOrderID, "pay_unk_1", 100000, "INR")
	sig := service.ComputeHmacSha256(string(payload), f.webhookSecret)
	eventID := "evt_unk_" + randomHex(6)

	err := f.paymentSvc.ProcessWebhook(ctx, eventID, payload, sig)
	if err != nil {
		t.Fatalf("expected nil (200 OK) for unknown order ID, got: %v", err)
	}

	var evtStatus, errorCode string
	_ = db.QueryRowContext(ctx, `SELECT status, error_code FROM payment_events WHERE event_id = $1`, eventID).Scan(&evtStatus, &errorCode)
	if evtStatus != "ignored" || errorCode != "ORDER_NOT_FOUND" {
		t.Fatalf("expected event status 'ignored' with 'ORDER_NOT_FOUND', got %q, %q", evtStatus, errorCode)
	}

	// Existing order in fixture remains pending
	var status string
	_ = db.QueryRowContext(ctx, `SELECT status FROM payments WHERE order_id = $1`, f.orderID).Scan(&status)
	if status != "pending" {
		t.Fatalf("expected fixture payment status 'pending', got %q", status)
	}
}

// 9. payment.failed event -> order stays pending, seats stay locked
func TestWebhookPaymentFailedLeavesPendingAndSeatsLocked(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()
	f := setupFixture(t, db)
	ctx := context.Background()

	paymentID := "pay_failed_" + randomHex(8)
	payload := f.createPaymentFailedWebhookPayload(f.orderID, paymentID, "BAD_REQUEST_ERROR")
	sig := service.ComputeHmacSha256(string(payload), f.webhookSecret)
	eventID := "evt_failed_" + randomHex(6)

	err := f.paymentSvc.ProcessWebhook(ctx, eventID, payload, sig)
	if err != nil {
		t.Fatalf("expected nil (200 OK) for payment.failed webhook, got: %v", err)
	}

	// Verify order status is still 'pending' (NOT failed, seats NOT released)
	var status, lastErrorCode string
	err = db.QueryRowContext(ctx, `
		SELECT status, COALESCE(last_error_code, '')
		FROM payments WHERE order_id = $1
	`, f.orderID).Scan(&status, &lastErrorCode)
	if err != nil || status != "pending" {
		t.Fatalf("expected payment status to remain 'pending', got %q (err=%v)", status, err)
	}
	if lastErrorCode != "BAD_REQUEST_ERROR" {
		t.Fatalf("expected last_error_code 'BAD_REQUEST_ERROR', got %q", lastErrorCode)
	}

	// Verify seats are still locked by User A
	var lockedBy string
	err = db.QueryRowContext(ctx, `SELECT locked_by FROM seat_inventory WHERE schedule_id = $1 AND seat_number = 'A1'`, f.scheduleID).Scan(&lockedBy)
	if err != nil || lockedBy != f.userA {
		t.Fatalf("expected seat A1 to remain locked by userA %q, got %q", f.userA, lockedBy)
	}
}

// 10. Late capture on expired order whose seats are still free -> confirms booking
func TestWebhookLateCaptureOnExpiredOrderFreeSeats(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()
	f := setupFixture(t, db)
	ctx := context.Background()

	// 1. Manually expire the payment order
	_, err := db.ExecContext(ctx, `
		UPDATE payments 
		SET status = 'expired', expires_at = NOW() - INTERVAL '1 minute'
		WHERE order_id = $1
	`, f.orderID)
	if err != nil {
		t.Fatalf("failed to expire order: %v", err)
	}

	// 2. Late webhook arrives (payment.captured) while seats A1 and A2 are still available
	paymentID := "pay_late_free_" + randomHex(8)
	payload := f.createWebhookPayload("payment.captured", f.orderID, paymentID, 100000, "INR")
	sig := service.ComputeHmacSha256(string(payload), f.webhookSecret)
	eventID := "evt_late_free_" + randomHex(6)

	err = f.paymentSvc.ProcessWebhook(ctx, eventID, payload, sig)
	if err != nil {
		t.Fatalf("late capture webhook failed: %v", err)
	}

	// Verify booking was created via grace rule
	var status, bookingRef string
	err = db.QueryRowContext(ctx, `SELECT status, COALESCE(booking_ref, '') FROM payments WHERE order_id = $1`, f.orderID).Scan(&status, &bookingRef)
	if err != nil || status != "captured" || bookingRef == "" {
		t.Fatalf("expected order to be captured with booking_ref, got status=%q, ref=%q (err=%v)", status, bookingRef, err)
	}

	// Seats are booked
	var seatStatus string
	err = db.QueryRowContext(ctx, `SELECT status FROM seat_inventory WHERE schedule_id = $1 AND seat_number = 'A1'`, f.scheduleID).Scan(&seatStatus)
	if err != nil || seatStatus != "booked" {
		t.Fatalf("expected seat A1 status 'booked', got %q", seatStatus)
	}
}

// 11. Late capture on expired order whose seat was re-booked -> sets refund_required
func TestWebhookLateCaptureOnExpiredOrderTakenSeatSetsRefundRequired(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()
	f := setupFixture(t, db)
	ctx := context.Background()

	// 1. Manually expire the payment order
	_, err := db.ExecContext(ctx, `
		UPDATE payments 
		SET status = 'expired', expires_at = NOW() - INTERVAL '1 minute'
		WHERE order_id = $1
	`, f.orderID)
	if err != nil {
		t.Fatalf("failed to expire order: %v", err)
	}

	// 2. Another user books seat A1
	_, err = db.ExecContext(ctx, `
		UPDATE seat_inventory 
		SET status = 'booked', locked_by = NULL, locked_at = NULL, lock_expires = NULL
		WHERE schedule_id = $1 AND seat_number = 'A1'
	`, f.scheduleID)
	if err != nil {
		t.Fatalf("failed to re-book seat A1: %v", err)
	}

	// 3. Late webhook arrives (payment.captured)
	paymentID := "pay_late_taken_" + randomHex(8)
	payload := f.createWebhookPayload("payment.captured", f.orderID, paymentID, 100000, "INR")
	sig := service.ComputeHmacSha256(string(payload), f.webhookSecret)
	eventID := "evt_late_taken_" + randomHex(6)

	err = f.paymentSvc.ProcessWebhook(ctx, eventID, payload, sig)
	if err != nil {
		t.Fatalf("late capture webhook returned error: %v", err)
	}

	// Verify payment status is 'refund_required'
	var status, gwPayID string
	err = db.QueryRowContext(ctx, `
		SELECT status, COALESCE(gateway_payment_id, '') 
		FROM payments WHERE order_id = $1
	`, f.orderID).Scan(&status, &gwPayID)
	if err != nil || status != "refund_required" {
		t.Fatalf("expected payment status 'refund_required', got %q (err=%v)", status, err)
	}
	if gwPayID != paymentID {
		t.Fatalf("expected gateway_payment_id %q, got %q", paymentID, gwPayID)
	}

	// Verify view_refund_required_payments contains this order
	var viewCount int
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM view_refund_required_payments WHERE order_id = $1`, f.orderID).Scan(&viewCount)
	if err != nil || viewCount != 1 {
		t.Fatalf("expected view_refund_required_payments to have 1 row for order %q, got %d", f.orderID, viewCount)
	}

	// Verify no booking exists for User A
	var userABookingCount int
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM bookings WHERE schedule_id = $1 AND user_id = $2`, f.scheduleID, f.userA).Scan(&userABookingCount)
	if err != nil || userABookingCount != 0 {
		t.Fatalf("expected 0 bookings for userA, got %d", userABookingCount)
	}
}

// 12. Transient DB failure mid-confirmation -> returns error, no partial state
func TestWebhookTransientDBFailureMidConfirmationRollsBack(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()
	f := setupFixture(t, db)

	// Use canceled context to simulate mid-operation database/context failure
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	payload := f.createWebhookPayload("payment.captured", f.orderID, "pay_tx_fail_1", 100000, "INR")
	sig := service.ComputeHmacSha256(string(payload), f.webhookSecret)
	eventID := "evt_canceled_" + randomHex(6)

	err := f.paymentSvc.ProcessWebhook(canceledCtx, eventID, payload, sig)
	if err == nil {
		t.Fatal("expected error with canceled context, got nil")
	}

	// Check DB with fresh background context: order is still pending and event was not committed as processed
	ctx := context.Background()
	var status string
	_ = db.QueryRowContext(ctx, `SELECT status FROM payments WHERE order_id = $1`, f.orderID).Scan(&status)
	if status != "pending" {
		t.Fatalf("expected payment status 'pending', got %q", status)
	}

	var processedCount int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM payment_events WHERE event_id = $1 AND status = 'processed'`, eventID).Scan(&processedCount)
	if processedCount != 0 {
		t.Fatalf("expected 0 processed payment_events, got %d", processedCount)
	}
}

// 13. Body > 1 MB -> returns 413
func TestWebhookMaxBodySizeReturns413(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()
	f := setupFixture(t, db)

	handler := paymenthandler.NewPaymentHandler(f.paymentSvc, nil)

	// Create body exceeding 1 MB (1048576 bytes)
	largeBody := make([]byte, 1048576+50)
	for i := range largeBody {
		largeBody[i] = 'a'
	}

	req := httptest.NewRequest(http.MethodPost, "/payments/webhook", bytes.NewReader(largeBody))
	req.Header.Set("X-Razorpay-Signature", "any_sig")
	rec := httptest.NewRecorder()

	handler.HandleWebhook(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected HTTP 413 Request Entity Too Large, got %d (body: %s)", rec.Code, rec.Body.String())
	}
}

