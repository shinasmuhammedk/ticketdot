package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"ticketdot/internal/httpx"
	"ticketdot/internal/notification"
	"ticketdot/internal/payment/gateway"
	"ticketdot/internal/payment/model"
	"ticketdot/internal/payment/repository"
)

var paymentIDRegex = regexp.MustCompile(`^pay_[A-Za-z0-9]{10,40}$`)

type keyLock struct {
	mu  sync.Mutex
	cnt int
}

type keyedMutex struct {
	mu    sync.Mutex
	locks map[string]*keyLock
}

func newKeyedMutex() *keyedMutex {
	return &keyedMutex{
		locks: make(map[string]*keyLock),
	}
}

func (km *keyedMutex) Lock(key string) func() {
	km.mu.Lock()
	l, ok := km.locks[key]
	if !ok {
		l = &keyLock{}
		km.locks[key] = l
	}
	l.cnt++
	km.mu.Unlock()

	l.mu.Lock()

	return func() {
		l.mu.Unlock()
		km.mu.Lock()
		l.cnt--
		if l.cnt == 0 {
			delete(km.locks, key)
		}
		km.mu.Unlock()
	}
}

type PaymentService struct {
	repo                 repository.PaymentRepository
	gateway              gateway.Gateway
	emailService         *notification.EmailService
	keyID                string
	keySecret            string
	webhookSecret        string
	paymentWindowMinutes int
	maxOrderAmountPaise  int64
	orderLocks           *keyedMutex
}

func (s *PaymentService) SetEmailService(es *notification.EmailService) {
	s.emailService = es
}

func (s *PaymentService) dispatchTicketEmailIfUnsent(ctx context.Context, booking *model.VerifyPaymentResponse) {
	if booking == nil || booking.ContactEmail == "" || s.emailService == nil {
		return
	}

	claimed, err := s.repo.ClaimTicketEmailDispatch(ctx, booking.OrderID)
	if err != nil || !claimed {
		return
	}

	defer func() { _ = recover() }()
	var manifest []notification.PassengerManifestItem
	for _, p := range booking.Passengers {
		manifest = append(manifest, notification.PassengerManifestItem{
			SeatNumber: p.SeatNumber,
			Name:       p.Name,
			Age:        p.Age,
			Gender:     p.Gender,
		})
	}
	pName := "Passenger"
	if len(booking.Passengers) > 0 && booking.Passengers[0].Name != "" {
		pName = booking.Passengers[0].Name
	}
	emailData := notification.TicketEmailData{
		BookingRef:    booking.BookingRef,
		PassengerName: pName,
		Origin:        booking.BoardingPoint,
		Destination:   booking.DroppingPoint,
		BoardingPoint: booking.BoardingPoint,
		DroppingPoint: booking.DroppingPoint,
		ContactPhone:  booking.ContactPhone,
		ContactEmail:  booking.ContactEmail,
		Seats:         booking.Seats,
		SeatsDisplay:  strings.Join(booking.Seats, ", "),
		Passengers:    manifest,
		Amount:        booking.TotalAmount,
		PaymentID:     booking.PaymentID,
		PaymentMethod: booking.PaymentMethod,
		PaymentStatus: "captured",
	}
	_ = s.emailService.SendTicketEmail(booking.ContactEmail, emailData)
}

func NewPaymentService(
	repo repository.PaymentRepository,
	gw gateway.Gateway,
	keyID, keySecret, webhookSecret string,
	windowMinutes int,
	maxAmount int64,
) *PaymentService {
	if windowMinutes <= 0 {
		windowMinutes = 12
	}
	if maxAmount <= 0 {
		maxAmount = 10000000 // default 100,000 INR = 10,000,000 paise
	}
	return &PaymentService{
		repo:                 repo,
		gateway:              gw,
		keyID:                strings.TrimSpace(keyID),
		keySecret:            strings.TrimSpace(keySecret),
		webhookSecret:        strings.TrimSpace(webhookSecret),
		paymentWindowMinutes: windowMinutes,
		maxOrderAmountPaise:  maxAmount,
		orderLocks:           newKeyedMutex(),
	}
}

func generateRandomID(prefix string, length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		b[i] = charset[n.Int64()]
	}
	return prefix + string(b), nil
}

func ComputeHmacSha256(message, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}

func (s *PaymentService) GenerateSignature(orderID, paymentID string) string {
	return ComputeHmacSha256(orderID+"|"+paymentID, s.keySecret)
}

func (s *PaymentService) GetKeyID() string {
	return s.keyID
}

func cleanSeats(seats []string) []string {
	var cleaned []string
	seen := make(map[string]bool)
	for _, seat := range seats {
		trimmed := strings.ToUpper(strings.TrimSpace(seat))
		if trimmed != "" && !seen[trimmed] {
			seen[trimmed] = true
			cleaned = append(cleaned, trimmed)
		}
	}
	sort.Strings(cleaned)
	return cleaned
}

func (s *PaymentService) CreateOrder(
	ctx context.Context,
	userID string,
	req model.CreateOrderRequest,
) (*model.CreateOrderResponse, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, httpx.Unauthorized("unauthorized")
	}

	req.ScheduleID = strings.TrimSpace(req.ScheduleID)
	if req.ScheduleID == "" {
		return nil, httpx.Validation("schedule id is required")
	}

	cleanedSeats := cleanSeats(req.Seats)
	if len(cleanedSeats) == 0 {
		return nil, httpx.Validation("at least one seat is required")
	}

	// Concurrency lock per (schedule_id, seats)
	lockKey := req.ScheduleID + ":" + strings.Join(cleanedSeats, ",")
	unlock := s.orderLocks.Lock(lockKey)
	defer unlock()

	// 1. Authoritative check: Verify that these seats are locked by this user and have not expired
	if err := s.repo.ValidateSeatLocks(ctx, req.ScheduleID, cleanedSeats, userID); err != nil {
		return nil, httpx.Validation(err.Error())
	}

	// 2. Fetch authoritative fare
	baseFare, _, _, err := s.repo.GetScheduleFare(ctx, req.ScheduleID)
	if err != nil {
		return nil, httpx.Validation(err.Error())
	}

	totalAmount := baseFare * int64(len(cleanedSeats))
	if totalAmount < 100 {
		return nil, httpx.Validation("order amount must be at least 100 paise (1 INR)")
	}
	if totalAmount > s.maxOrderAmountPaise {
		return nil, httpx.Validation(fmt.Sprintf("order amount exceeds maximum allowed limit of %d paise", s.maxOrderAmountPaise))
	}

	// 3. Idempotency & existing order check
	existing, err := s.repo.FindPendingOrderByScheduleAndSeats(ctx, req.ScheduleID, cleanedSeats)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing orders: %w", err)
	}

	if existing != nil {
		if existing.UserID != userID {
			if existing.ExpiresAt == nil || existing.ExpiresAt.After(time.Now()) {
				return nil, httpx.Conflict("seat order already in progress by another user")
			}
			_ = s.repo.ExpirePaymentOrder(ctx, existing.ID)
		} else {
			if existing.ExpiresAt != nil && existing.ExpiresAt.After(time.Now()) {
				// Re-verify that seat locks are still held by this user
				if err := s.repo.ValidateSeatLocks(ctx, req.ScheduleID, cleanedSeats, userID); err != nil {
					return nil, httpx.Validation(err.Error())
				}
				expiresAtStr := ""
				if existing.ExpiresAt != nil {
					expiresAtStr = existing.ExpiresAt.Format(time.RFC3339)
				}
				return &model.CreateOrderResponse{
					OrderID:     existing.OrderID,
					KeyID:       s.keyID,
					Amount:      existing.Amount,
					AmountPaise: existing.Amount,
					Currency:    existing.Currency,
					ScheduleID:  existing.ScheduleID,
					Seats:       existing.Seats,
					CreatedAt:   existing.CreatedAt.Format(time.RFC3339),
					ExpiresAt:   expiresAtStr,
				}, nil
			}
			_ = s.repo.ExpirePaymentOrder(ctx, existing.ID)
		}
	}

	// 4. Generate unique receipt (max 40 chars)
	receiptSuffix, err := generateRandomID("", 8)
	if err != nil {
		return nil, fmt.Errorf("failed to generate receipt ID: %w", err)
	}
	receipt := fmt.Sprintf("rcpt_%d_%s", time.Now().Unix(), receiptSuffix)
	if len(receipt) > 40 {
		receipt = receipt[:40]
	}

	// 5. Create Order on Payment Gateway (Razorpay or Fake)
	notes := map[string]string{
		"schedule_id": req.ScheduleID,
		"seats":       strings.Join(cleanedSeats, ","),
		"user_id":     userID,
	}
	gwOrder, err := s.gateway.CreateOrder(ctx, gateway.CreateOrderInput{
		AmountPaise: totalAmount,
		Currency:    "INR",
		Receipt:     receipt,
		Notes:       notes,
	})
	if err != nil {
		var appErr *httpx.AppError
		if errors.As(err, &appErr) {
			return nil, appErr
		}
		return nil, httpx.BadGateway("payment gateway error: failed to create order")
	}

	gatewayName := "razorpay"
	if strings.HasPrefix(gwOrder.ID, "order_fake_") {
		gatewayName = "fake"
	}

	payment := &model.Payment{
		OrderID:     gwOrder.ID,
		UserID:      userID,
		ScheduleID:  req.ScheduleID,
		Seats:       cleanedSeats,
		Amount:      totalAmount,
		AmountPaise: totalAmount,
		Currency:    gwOrder.Currency,
		Status:      "pending",
		Gateway:     gatewayName,
	}

	// 6. Atomically lock seat inventory rows, extend locks and insert payment record
	if err := s.repo.CreateOrderAndExtendLocks(ctx, payment, s.paymentWindowMinutes); err != nil {
		return nil, err
	}

	expiresAtStr := ""
	if payment.ExpiresAt != nil {
		expiresAtStr = payment.ExpiresAt.Format(time.RFC3339)
	}

	return &model.CreateOrderResponse{
		OrderID:     payment.OrderID,
		KeyID:       s.keyID,
		Amount:      payment.Amount,
		AmountPaise: payment.Amount,
		Currency:    payment.Currency,
		ScheduleID:  payment.ScheduleID,
		Seats:       payment.Seats,
		CreatedAt:   payment.CreatedAt.Format(time.RFC3339),
		ExpiresAt:   expiresAtStr,
	}, nil
}

func (s *PaymentService) GetGateway() gateway.Gateway {
	return s.gateway
}

func (s *PaymentService) RegisterTestPaymentIfFake(ctx context.Context, orderID, paymentID string) error {
	fakeGw, ok := s.gateway.(*gateway.FakeGateway)
	if !ok {
		return nil
	}

	order, err := s.repo.GetPaymentByOrderID(ctx, orderID)
	if err != nil || order == nil {
		fakeGw.RegisterPayment(paymentID, orderID, 100000, "INR", "captured", "card")
		return nil
	}

	fakeGw.RegisterPayment(paymentID, orderID, order.Amount, order.Currency, "captured", "card")
	return nil
}

func (s *PaymentService) VerifyPayment(
	ctx context.Context,
	userID string,
	req model.VerifyPaymentRequest,
) (*model.VerifyPaymentResponse, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, httpx.Unauthorized("unauthorized")
	}

	req.OrderID = strings.TrimSpace(req.OrderID)
	req.PaymentID = strings.TrimSpace(req.PaymentID)
	req.Signature = strings.TrimSpace(req.Signature)

	if req.OrderID == "" || req.PaymentID == "" {
		return nil, httpx.Validation("order_id and payment_id are required")
	}

	// Step 1: Order lookup (NO long locks)
	order, err := s.repo.GetPaymentByOrderID(ctx, req.OrderID)
	if err != nil || order == nil || order.UserID != userID {
		return nil, httpx.NotFound("payment not found")
	}

	// Step 2: HMAC SHA-256 signature check
	expectedSignature := ComputeHmacSha256(req.OrderID+"|"+req.PaymentID, s.keySecret)
	if !hmac.Equal([]byte(strings.ToLower(req.Signature)), []byte(strings.ToLower(expectedSignature))) {
		_ = s.repo.RecordPaymentLastError(ctx, req.OrderID, "SIGNATURE_MISMATCH")
		log.Printf("[Payment Verification Error] signature mismatch for order %s", req.OrderID)
		return nil, httpx.Validation("invalid payment signature: payment verification failed")
	}

	// Step 3: Idempotency check if already captured
	if order.Status == "captured" {
		capturedID := ""
		if order.GatewayPaymentID != nil && *order.GatewayPaymentID != "" {
			capturedID = *order.GatewayPaymentID
		} else if order.PaymentID != nil && *order.PaymentID != "" {
			capturedID = *order.PaymentID
		}
		if capturedID != "" && capturedID != req.PaymentID {
			return nil, httpx.BadRequest("payment order has already been captured with a different payment ID")
		}
		req.Seats = cleanSeats(req.Seats)
		return s.repo.VerifyAndCapture(ctx, userID, req, 0)
	}

	// Step 4: Call gateway.FetchPayment OUTSIDE any database transaction
	if !paymentIDRegex.MatchString(req.PaymentID) {
		return nil, httpx.Validation("invalid payment id format")
	}

	gwPayment, err := s.gateway.FetchPayment(ctx, req.PaymentID)
	if err != nil {
		var appErr *httpx.AppError
		if errors.As(err, &appErr) {
			return nil, appErr
		}
		return nil, httpx.BadGateway("payment gateway error during payment verification")
	}

	// Cross-check payment fields against stored order
	if gwPayment.OrderID != order.OrderID {
		_ = s.repo.RecordPaymentLastError(ctx, req.OrderID, "ORDER_ID_MISMATCH")
		log.Printf("[Payment Verification Error] order ID mismatch: gateway has %q, order is %q", gwPayment.OrderID, order.OrderID)
		return nil, httpx.BadRequest("payment order ID does not match gateway order")
	}

	if gwPayment.AmountPaise != order.Amount {
		_ = s.repo.RecordPaymentLastError(ctx, req.OrderID, "AMOUNT_MISMATCH")
		log.Printf("[Payment Verification Error] amount mismatch: gateway has %d, order is %d", gwPayment.AmountPaise, order.Amount)
		return nil, httpx.BadRequest("payment amount does not match order amount")
	}

	if !strings.EqualFold(gwPayment.Currency, order.Currency) {
		_ = s.repo.RecordPaymentLastError(ctx, req.OrderID, "CURRENCY_MISMATCH")
		log.Printf("[Payment Verification Error] currency mismatch: gateway has %q, order is %q", gwPayment.Currency, order.Currency)
		return nil, httpx.BadRequest("payment currency does not match order currency")
	}

	// Handle gateway payment status
	switch strings.ToLower(gwPayment.Status) {
	case "authorized":
		// Gateway authorized but not yet captured: respond with HTTP 202 Accepted {"status": "processing"}
		return &model.VerifyPaymentResponse{
			OrderID:          order.OrderID,
			PaymentID:        req.PaymentID,
			Status:           "processing",
			TotalAmount:      order.Amount,
			TotalAmountPaise: order.Amount,
			Currency:         order.Currency,
			PaymentMethod:    gwPayment.Method,
			ScheduleID:       order.ScheduleID,
			Seats:            order.Seats,
		}, nil

	case "captured":
		// Proceed to atomic DB capture transaction below

	case "failed":
		_ = s.repo.RecordPaymentLastError(ctx, req.OrderID, "PAYMENT_FAILED")
		return nil, httpx.BadRequest("payment has failed on payment gateway")

	case "created":
		_ = s.repo.RecordPaymentLastError(ctx, req.OrderID, "PAYMENT_CREATED")
		return nil, httpx.BadRequest("payment has not been authorized or completed on payment gateway")

	default:
		_ = s.repo.RecordPaymentLastError(ctx, req.OrderID, "UNKNOWN_PAYMENT_STATUS")
		return nil, httpx.BadRequest(fmt.Sprintf("unsupported payment status: %s", gwPayment.Status))
	}

	req.Seats = cleanSeats(req.Seats)

	// Step 5: Perform atomic capture & seat booking in DB transaction
	confirmIn := model.ConfirmPaymentInput{
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

	result, err := s.repo.ConfirmCapturedPayment(ctx, confirmIn)
	if err != nil {
		var appErr *httpx.AppError
		if errors.As(err, &appErr) {
			return nil, appErr
		}
		return nil, err
	}

	if result.Status == model.ConfirmationStatusConfirmed && result.Booking != nil {
		go s.dispatchTicketEmailIfUnsent(context.Background(), result.Booking)
	}

	if result.Status == model.ConfirmationStatusNotBookable {
		var amt int64
		var curr, sched string
		var seats []string
		if result.Payment != nil {
			amt = result.Payment.Amount
			curr = result.Payment.Currency
			sched = result.Payment.ScheduleID
			seats = result.Payment.Seats
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

	return result.Booking, nil
}

func (s *PaymentService) ConfirmCapturedPayment(ctx context.Context, in model.ConfirmPaymentInput) (*model.ConfirmationResult, error) {
	res, err := s.repo.ConfirmCapturedPayment(ctx, in)
	if err != nil {
		return nil, err
	}
	if res.Status == model.ConfirmationStatusConfirmed && res.Booking != nil {
		go s.dispatchTicketEmailIfUnsent(context.Background(), res.Booking)
	}
	return res, nil
}

func (s *PaymentService) GetPaymentStatus(ctx context.Context, userID, orderID string) (*model.PaymentStatusResponse, error) {
	p, err := s.repo.GetPaymentByOrderID(ctx, strings.TrimSpace(orderID))
	if err != nil || p == nil {
		return nil, errors.New("payment not found")
	}

	if p.UserID != strings.TrimSpace(userID) {
		return nil, errors.New("payment not found")
	}

	refundPending := p.Status == "refund_required" || (p.RefundStatus != nil && *p.RefundStatus == "pending")

	return &model.PaymentStatusResponse{
		OrderID:           p.OrderID,
		PaymentID:         p.PaymentID,
		GatewayPaymentID:  p.GatewayPaymentID,
		BookingRef:        p.BookingRef,
		Status:            p.Status,
		Amount:            p.Amount,
		AmountPaise:       p.Amount,
		Currency:          p.Currency,
		PaymentMethod:     p.PaymentMethod,
		RefundID:          p.RefundID,
		RefundAmount:      p.RefundAmount,
		RefundAmountPaise: p.RefundAmount,
		RefundStatus:      p.RefundStatus,
		RefundedAt:        p.RefundedAt,
		RefundPending:     refundPending,
		CreatedAt:         p.CreatedAt,
	}, nil
}

func (s *PaymentService) ProcessWebhook(ctx context.Context, eventID string, rawBody []byte, signature string) error {
	signature = strings.TrimSpace(signature)
	if signature == "" {
		return httpx.Unauthorized("missing webhook signature")
	}

	expectedSignature := ComputeHmacSha256(string(rawBody), s.webhookSecret)
	if !hmac.Equal([]byte(strings.ToLower(signature)), []byte(strings.ToLower(expectedSignature))) {
		return httpx.Unauthorized("invalid webhook signature")
	}

	// Payload is cryptographically verified; parse JSON AFTER signature verification
	var payload model.WebhookPayload
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		log.Printf("[WARN] invalid webhook JSON payload: %v", err)
		return httpx.BadRequest("invalid json payload")
	}

	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		eventID = strings.TrimSpace(payload.EventID)
	}
	if eventID == "" {
		eventID = "evt_" + ComputeHmacSha256(string(rawBody), s.keySecret)[:16]
	}

	// Dedupe by X-Razorpay-Event-Id header. Insert event row and run business logic in SAME transaction.
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return httpx.Internal(err)
	}
	defer tx.Rollback()

	inserted, err := s.repo.RecordPaymentEventInTx(ctx, tx, eventID, payload.Event, "", "", "processed", "")
	if err != nil {
		return httpx.Internal(err)
	}
	if !inserted {
		// Duplicate event id returns 200 and is ignored.
		_ = tx.Commit()
		return nil
	}

	switch payload.Event {
	case "payment.captured", "order.paid":
		orderID := strings.TrimSpace(payload.Payload.Payment.Entity.OrderID)
		if orderID == "" {
			orderID = strings.TrimSpace(payload.Payload.Order.Entity.ID)
		}
		paymentID := strings.TrimSpace(payload.Payload.Payment.Entity.ID)
		amount := payload.Payload.Payment.Entity.Amount
		if amount == 0 {
			amount = payload.Payload.Order.Entity.Amount
		}
		currency := strings.ToUpper(strings.TrimSpace(payload.Payload.Payment.Entity.Currency))
		if currency == "" {
			currency = strings.ToUpper(strings.TrimSpace(payload.Payload.Order.Entity.Currency))
		}
		method := strings.TrimSpace(payload.Payload.Payment.Entity.Method)

		if orderID == "" {
			_ = s.repo.UpdatePaymentEventInTx(ctx, tx, eventID, "ignored", "MISSING_ORDER_ID", "", paymentID)
			return tx.Commit()
		}

		order, err := s.repo.GetPaymentByOrderID(ctx, orderID)
		if err != nil || order == nil {
			// Unknown order: the Razorpay account may carry other orders. Record as ignored and return 200.
			_ = s.repo.UpdatePaymentEventInTx(ctx, tx, eventID, "ignored", "ORDER_NOT_FOUND", orderID, paymentID)
			return tx.Commit()
		}

		// Cross-check amount (paise) and currency against stored order
		if order.Amount != amount {
			log.Printf("[WARN] webhook amount mismatch for order %s: stored=%d, event=%d", order.OrderID, order.Amount, amount)
			_ = s.repo.RecordPaymentLastErrorInTx(ctx, tx, order.OrderID, "AMOUNT_MISMATCH")
			_ = s.repo.UpdatePaymentEventInTx(ctx, tx, eventID, "ignored", "AMOUNT_MISMATCH", order.OrderID, paymentID)
			return tx.Commit()
		}
		if currency != "" && !strings.EqualFold(order.Currency, currency) {
			log.Printf("[WARN] webhook currency mismatch for order %s: stored=%s, event=%s", order.OrderID, order.Currency, currency)
			_ = s.repo.RecordPaymentLastErrorInTx(ctx, tx, order.OrderID, "CURRENCY_MISMATCH")
			_ = s.repo.UpdatePaymentEventInTx(ctx, tx, eventID, "ignored", "CURRENCY_MISMATCH", order.OrderID, paymentID)
			return tx.Commit()
		}

		confirmIn := model.ConfirmPaymentInput{
			OrderID:          order.OrderID,
			PaymentID:        paymentID,
			GatewayPaymentID: paymentID,
			AmountPaise:      amount,
			Currency:         order.Currency,
			PaymentMethod:    method,
		}

		res, err := s.repo.ConfirmCapturedPaymentInTx(ctx, tx, confirmIn)
		if err != nil {
			// Transient failure: rollback and return 500 so Razorpay retries
			return httpx.Internal(err)
		}

		_ = s.repo.UpdatePaymentEventInTx(ctx, tx, eventID, "processed", "", order.OrderID, paymentID)
		if err := tx.Commit(); err != nil {
			return httpx.Internal(err)
		}

		if res.Status == model.ConfirmationStatusConfirmed && res.Booking != nil {
			go s.dispatchTicketEmailIfUnsent(context.Background(), res.Booking)
		}
		return nil

	case "payment.failed":
		orderID := strings.TrimSpace(payload.Payload.Payment.Entity.OrderID)
		paymentID := strings.TrimSpace(payload.Payload.Payment.Entity.ID)
		errorCode := strings.TrimSpace(payload.Payload.Payment.Entity.ErrorCode)
		if errorCode == "" {
			errorCode = "PAYMENT_FAILED"
		}

		if orderID != "" {
			_ = s.repo.RecordPaymentLastErrorInTx(ctx, tx, orderID, errorCode)
		}
		_ = s.repo.UpdatePaymentEventInTx(ctx, tx, eventID, "processed", errorCode, orderID, paymentID)
		return tx.Commit()

	case "payment.authorized":
		// Documented: account must use automatic capture; do not implement capture API call
		orderID := strings.TrimSpace(payload.Payload.Payment.Entity.OrderID)
		paymentID := strings.TrimSpace(payload.Payload.Payment.Entity.ID)
		_ = s.repo.UpdatePaymentEventInTx(ctx, tx, eventID, "ignored", "AUTHORIZED_ONLY_AUTOMATIC_CAPTURE_REQUIRED", orderID, paymentID)
		return tx.Commit()

	default:
		// Unsupported event type: record as ignored, return 200
		_ = s.repo.UpdatePaymentEventInTx(ctx, tx, eventID, "ignored", "UNSUPPORTED_EVENT_TYPE", "", "")
		return tx.Commit()
	}
}
