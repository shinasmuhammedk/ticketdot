package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"ticketdot/internal/payment/model"
	"ticketdot/internal/payment/repository"
)

type PaymentService struct {
	repo          repository.PaymentRepository
	keyID         string
	keySecret     string
	webhookSecret string
}

func NewPaymentService(repo repository.PaymentRepository) *PaymentService {
	keyID := strings.TrimSpace(os.Getenv("RAZORPAY_KEY_ID"))
	if keyID == "" {
		keyID = "rzp_test_TicketDotDemo"
	}

	keySecret := strings.TrimSpace(os.Getenv("RAZORPAY_KEY_SECRET"))
	if keySecret == "" {
		keySecret = "rzp_test_secret_TicketDotDemo"
	}

	webhookSecret := strings.TrimSpace(os.Getenv("PAYMENT_WEBHOOK_SECRET"))
	if webhookSecret == "" {
		webhookSecret = "webhook_secret_TicketDotDemo"
	}

	return &PaymentService{
		repo:          repo,
		keyID:         keyID,
		keySecret:     keySecret,
		webhookSecret: webhookSecret,
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
	return cleaned
}

func (s *PaymentService) CreateOrder(
	ctx context.Context,
	userID string,
	req model.CreateOrderRequest,
) (*model.CreateOrderResponse, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, errors.New("user id is required")
	}

	req.ScheduleID = strings.TrimSpace(req.ScheduleID)
	if req.ScheduleID == "" {
		return nil, errors.New("schedule id is required")
	}

	cleanedSeats := cleanSeats(req.Seats)
	if len(cleanedSeats) == 0 {
		return nil, errors.New("at least one seat is required")
	}

	// 1. Authoritative check: Verify that these seats are locked by this user and have not expired
	if err := s.repo.ValidateSeatLocks(ctx, req.ScheduleID, cleanedSeats, userID); err != nil {
		return nil, err
	}

	// 2. Fetch authoritative fare
	baseFare, _, _, err := s.repo.GetScheduleFare(ctx, req.ScheduleID)
	if err != nil {
		return nil, err
	}

	totalAmount := baseFare * float64(len(cleanedSeats))
	orderID, err := generateRandomID("order_", 16)
	if err != nil {
		return nil, fmt.Errorf("failed to generate order id: %w", err)
	}

	payment := &model.Payment{
		OrderID:    orderID,
		UserID:     userID,
		ScheduleID: req.ScheduleID,
		Seats:      cleanedSeats,
		Amount:     totalAmount,
		Currency:   "INR",
		Status:     "pending",
		Gateway:    "razorpay",
	}

	if err := s.repo.CreateOrder(ctx, payment); err != nil {
		return nil, fmt.Errorf("failed to save payment order: %w", err)
	}

	return &model.CreateOrderResponse{
		OrderID:    orderID,
		ScheduleID: req.ScheduleID,
		Seats:      cleanedSeats,
		Amount:     totalAmount,
		Currency:   "INR",
		KeyID:      s.keyID,
		CreatedAt:  payment.CreatedAt.Format(time.RFC3339),
	}, nil
}

func (s *PaymentService) VerifyPayment(
	ctx context.Context,
	userID string,
	req model.VerifyPaymentRequest,
) (*model.VerifyPaymentResponse, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, errors.New("user id is required")
	}

	req.OrderID = strings.TrimSpace(req.OrderID)
	req.PaymentID = strings.TrimSpace(req.PaymentID)
	req.Signature = strings.TrimSpace(req.Signature)

	if req.OrderID == "" || req.PaymentID == "" {
		return nil, errors.New("order_id and payment_id are required")
	}

	// 1. Verify HMAC SHA-256 signature
	expectedSignature := ComputeHmacSha256(req.OrderID+"|"+req.PaymentID, s.keySecret)
	if !hmac.Equal([]byte(strings.ToLower(req.Signature)), []byte(strings.ToLower(expectedSignature))) {
		// Log and record signature tampering/failure in payments table
		_ = s.repo.RecordPaymentFailure(ctx, req.OrderID, req.PaymentID, "SIGNATURE_MISMATCH", "Invalid payment gateway signature")
		return nil, errors.New("invalid payment signature: payment verification failed")
	}

	req.Seats = cleanSeats(req.Seats)
	if len(req.Seats) == 0 {
		return nil, errors.New("at least one seat is required")
	}

	// 2. Fetch authoritative fare
	baseFare, _, _, err := s.repo.GetScheduleFare(ctx, req.ScheduleID)
	if err != nil {
		return nil, err
	}

	// 3. Perform atomic capture & seat booking
	res, err := s.repo.VerifyAndCapture(ctx, userID, req, baseFare)
	if err != nil {
		_ = s.repo.RecordPaymentFailure(ctx, req.OrderID, req.PaymentID, "CAPTURE_FAILED", err.Error())
		return nil, err
	}

	return res, nil
}

func (s *PaymentService) GetPaymentStatus(ctx context.Context, orderID string) (*model.PaymentStatusResponse, error) {
	p, err := s.repo.GetPaymentByOrderID(ctx, strings.TrimSpace(orderID))
	if err != nil {
		return nil, errors.New("payment not found")
	}

	return &model.PaymentStatusResponse{
		OrderID:       p.OrderID,
		PaymentID:     p.PaymentID,
		BookingRef:    p.BookingRef,
		Status:        p.Status,
		Amount:        p.Amount,
		Currency:      p.Currency,
		PaymentMethod: p.PaymentMethod,
		RefundID:      p.RefundID,
		RefundAmount:  p.RefundAmount,
		RefundStatus:  p.RefundStatus,
		RefundedAt:    p.RefundedAt,
		CreatedAt:     p.CreatedAt,
	}, nil
}

func (s *PaymentService) ProcessWebhook(ctx context.Context, rawBody []byte, signature string) error {
	signature = strings.TrimSpace(signature)
	if signature == "" {
		return errors.New("missing webhook signature")
	}

	expectedSignature := ComputeHmacSha256(string(rawBody), s.webhookSecret)
	if !hmac.Equal([]byte(strings.ToLower(signature)), []byte(strings.ToLower(expectedSignature))) {
		return errors.New("invalid webhook signature")
	}

	// Payload is cryptographically verified
	return nil
}
