package routes_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jmoiron/sqlx"

	authhandler "ticketdot/internal/auth/handler"
	bookinghandler "ticketdot/internal/booking/handler"
	"ticketdot/internal/config"
	healthhandler "ticketdot/internal/health"
	operatorhandler "ticketdot/internal/operator/handler"
	paymentgateway "ticketdot/internal/payment/gateway"
	paymenthandler "ticketdot/internal/payment/handler"
	paymentmodel "ticketdot/internal/payment/model"
	paymentservice "ticketdot/internal/payment/service"
	profilehandler "ticketdot/internal/profile/handler"
	"ticketdot/internal/routes"
	searchhandler "ticketdot/internal/search/handler"
	seathandler "ticketdot/internal/seat/handler"
	trackinghandler "ticketdot/internal/tracking/handler"
	trackingmodel "ticketdot/internal/tracking/model"
	trackingservice "ticketdot/internal/tracking/service"
	reviewhandler "ticketdot/internal/review/handler"
)

// Mock tracking repository
type mockTrackingRepo struct {
	routeInfo *trackingmodel.ScheduleRouteInfo
}

func (m *mockTrackingRepo) GetTrackingByScheduleID(ctx context.Context, scheduleID string) (*trackingmodel.TripTracking, error) {
	return nil, nil
}

func (m *mockTrackingRepo) UpsertTracking(ctx context.Context, tracking *trackingmodel.TripTracking) error {
	return nil
}

func (m *mockTrackingRepo) GetScheduleRouteInfo(ctx context.Context, scheduleID string) (*trackingmodel.ScheduleRouteInfo, error) {
	if m.routeInfo != nil {
		return m.routeInfo, nil
	}
	return &trackingmodel.ScheduleRouteInfo{
		ScheduleID:    scheduleID,
		OperatorID:    "op-owner-1",
		OriginLat:     12.9716,
		OriginLng:     77.5946,
		DestLat:       13.0827,
		DestLng:       80.2707,
		DepartureTime: time.Now(),
		ArrivalTime:   time.Now().Add(4 * time.Hour),
	}, nil
}

func (m *mockTrackingRepo) GetScheduleIDByBookingRef(ctx context.Context, bookingRef string) (string, error) {
	return "sched-1", nil
}

// Mock payment repository
type mockPaymentRepo struct{}

func (m *mockPaymentRepo) CreateOrder(ctx context.Context, p *paymentmodel.Payment) error { return nil }
func (m *mockPaymentRepo) CreateOrderAndExtendLocks(ctx context.Context, p *paymentmodel.Payment, windowMinutes int) error {
	return nil
}
func (m *mockPaymentRepo) FindPendingOrderByScheduleAndSeats(ctx context.Context, scheduleID string, seats []string) (*paymentmodel.Payment, error) {
	return nil, nil
}
func (m *mockPaymentRepo) ExpirePaymentOrder(ctx context.Context, id string) error {
	return nil
}
func (m *mockPaymentRepo) GetPaymentByOrderID(ctx context.Context, orderID string) (*paymentmodel.Payment, error) {
	return nil, nil
}
func (m *mockPaymentRepo) GetPaymentByBookingRef(ctx context.Context, bookingRef string) (*paymentmodel.Payment, error) {
	return nil, nil
}
func (m *mockPaymentRepo) VerifyAndCapture(ctx context.Context, userID string, req paymentmodel.VerifyPaymentRequest, amount int64) (*paymentmodel.VerifyPaymentResponse, error) {
	return nil, nil
}
func (m *mockPaymentRepo) RecordPaymentFailure(ctx context.Context, orderID, paymentID, userID, errorCode, errorDesc string) error {
	return nil
}
func (m *mockPaymentRepo) RecordPaymentLastError(ctx context.Context, orderID, errorCode string) error {
	return nil
}
func (m *mockPaymentRepo) ExpireStalePendingOrders(ctx context.Context) (int64, error) {
	return 0, nil
}
func (m *mockPaymentRepo) ProcessRefund(ctx context.Context, bookingRef, refundID string, refundAmount int64) error {
	return nil
}
func (m *mockPaymentRepo) GetScheduleFare(ctx context.Context, scheduleID string) (int64, string, string, error) {
	return 50000, "City A", "City B", nil
}
func (m *mockPaymentRepo) ValidateSeatLocks(ctx context.Context, scheduleID string, seats []string, userID string) error {
	return nil
}
func (m *mockPaymentRepo) ConfirmCapturedPayment(ctx context.Context, in paymentmodel.ConfirmPaymentInput) (*paymentmodel.ConfirmationResult, error) {
	return &paymentmodel.ConfirmationResult{Status: paymentmodel.ConfirmationStatusConfirmed}, nil
}
func (m *mockPaymentRepo) ConfirmCapturedPaymentInTx(ctx context.Context, tx *sqlx.Tx, in paymentmodel.ConfirmPaymentInput) (*paymentmodel.ConfirmationResult, error) {
	return &paymentmodel.ConfirmationResult{Status: paymentmodel.ConfirmationStatusConfirmed}, nil
}
func (m *mockPaymentRepo) RecordPaymentLastErrorInTx(ctx context.Context, tx *sqlx.Tx, orderID, errorCode string) error {
	return nil
}
func (m *mockPaymentRepo) ClaimTicketEmailDispatch(ctx context.Context, orderID string) (bool, error) {
	return true, nil
}
func (m *mockPaymentRepo) RecordPaymentEventInTx(ctx context.Context, tx *sqlx.Tx, eventID, eventType, orderID, paymentID, status, errorCode string) (bool, error) {
	return true, nil
}
func (m *mockPaymentRepo) UpdatePaymentEventInTx(ctx context.Context, tx *sqlx.Tx, eventID, status, errorCode, orderID, paymentID string) error {
	return nil
}
func (m *mockPaymentRepo) GetPaymentEventByEventID(ctx context.Context, eventID string) (*paymentmodel.PaymentEvent, error) {
	return nil, nil
}
func (m *mockPaymentRepo) ListRefundRequiredPayments(ctx context.Context) ([]paymentmodel.Payment, error) {
	return nil, nil
}
func (m *mockPaymentRepo) BeginTx(ctx context.Context) (*sqlx.Tx, error) {
	return nil, nil
}

func createTestToken(t *testing.T, secret string, userID, role string, operatorID *string) string {
	claims := jwt.MapClaims{
		"user_id": userID,
		"role":    role,
		"exp":     time.Now().Add(time.Hour).Unix(),
	}
	if operatorID != nil {
		claims["operator_id"] = *operatorID
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return tokenStr
}

func buildTestRouter(cfg *config.Config, trackingRepo *mockTrackingRepo) http.Handler {
	paySvc := paymentservice.NewPaymentService(&mockPaymentRepo{}, paymentgateway.NewFakeGateway(), "key", "secret", "webhook", 12, 10000000)
	payH := paymenthandler.NewPaymentHandler(paySvc, nil)

	trackSvc := trackingservice.NewTrackingService(trackingRepo)
	trackH := trackinghandler.NewTrackingHandler(trackSvc)

	return routes.SetupRoutes(routes.Handlers{
		Config:   cfg,
		Health:   healthhandler.NewHealthHandler(nil),
		Auth:     &authhandler.AuthHandler{},
		Search:   &searchhandler.SearchHandler{},
		Seat:     &seathandler.SeatHandler{},
		Booking:  &bookinghandler.BookingHandler{},
		Operator: &operatorhandler.OperatorHandler{},
		Profile:  &profilehandler.ProfileHandler{},
		Payment:  payH,
		Tracking: trackH,
		Review:   &reviewhandler.ReviewHandler{},
	})
}

func TestTestSignatureRouteOnlyInDevelopment(t *testing.T) {
	// Case 1: APP_ENV = production -> Route must return 404
	prodCfg := &config.Config{
		AppEnv: "production",
	}
	prodRouter := buildTestRouter(prodCfg, &mockTrackingRepo{})

	reqBody, _ := json.Marshal(map[string]string{
		"order_id":   "order_123",
		"payment_id": "pay_123",
	})
	req := httptest.NewRequest(http.MethodPost, "/payments/test-signature", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	prodRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for /payments/test-signature in production, got %d", rec.Code)
	}

	// Case 2: APP_ENV = unset / "" -> Route must return 404
	unsetCfg := &config.Config{
		AppEnv: "",
	}
	unsetRouter := buildTestRouter(unsetCfg, &mockTrackingRepo{})
	recUnset := httptest.NewRecorder()
	unsetRouter.ServeHTTP(recUnset, req)

	if recUnset.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for /payments/test-signature when APP_ENV unset, got %d", recUnset.Code)
	}

	// Case 3: APP_ENV = development -> Route exists (200 with signature)
	devCfg := &config.Config{
		AppEnv: "development",
	}
	devRouter := buildTestRouter(devCfg, &mockTrackingRepo{})
	recDev := httptest.NewRecorder()
	devRouter.ServeHTTP(recDev, req)

	if recDev.Code != http.StatusOK {
		t.Fatalf("expected 200 for /payments/test-signature in development, got %d: %s", recDev.Code, recDev.Body.String())
	}
}

func TestSimulateTripAuthorization(t *testing.T) {
	jwtSecret := "super-secret-key-32-chars-long-for-testing"
	os.Setenv("JWT_SECRET", jwtSecret)
	defer os.Unsetenv("JWT_SECRET")

	trackingRepo := &mockTrackingRepo{
		routeInfo: &trackingmodel.ScheduleRouteInfo{
			ScheduleID:    "sched-1",
			OperatorID:    "op-owner-1",
			OriginLat:     12.9716,
			OriginLng:     77.5946,
			DestLat:       13.0827,
			DestLng:       80.2707,
			DepartureTime: time.Now(),
			ArrivalTime:   time.Now().Add(4 * time.Hour),
		},
	}

	cfg := &config.Config{
		AppEnv: "development",
	}
	router := buildTestRouter(cfg, trackingRepo)

	bodyBytes, _ := json.Marshal(map[string]string{
		"action": "step",
	})

	// 1. Unauthenticated request: returns 401
	reqUnauth := httptest.NewRequest(http.MethodPost, "/trips/sched-1/simulate", bytes.NewReader(bodyBytes))
	recUnauth := httptest.NewRecorder()
	router.ServeHTTP(recUnauth, reqUnauth)

	if recUnauth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated simulate, got %d: %s", recUnauth.Code, recUnauth.Body.String())
	}

	// 2. Request with passenger role token: returns 403
	passengerToken := createTestToken(t, jwtSecret, "user-passenger-1", "passenger", nil)
	reqPassenger := httptest.NewRequest(http.MethodPost, "/trips/sched-1/simulate", bytes.NewReader(bodyBytes))
	reqPassenger.Header.Set("Authorization", "Bearer "+passengerToken)
	recPassenger := httptest.NewRecorder()
	router.ServeHTTP(recPassenger, reqPassenger)

	if recPassenger.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for passenger role on simulate, got %d: %s", recPassenger.Code, recPassenger.Body.String())
	}

	// 3. Request with different operator token: returns 403
	differentOp := "op-intruder-99"
	diffOpToken := createTestToken(t, jwtSecret, "user-op-2", "operator_admin", &differentOp)
	reqDiffOp := httptest.NewRequest(http.MethodPost, "/trips/sched-1/simulate", bytes.NewReader(bodyBytes))
	reqDiffOp.Header.Set("Authorization", "Bearer "+diffOpToken)
	recDiffOp := httptest.NewRecorder()
	router.ServeHTTP(recDiffOp, reqDiffOp)

	if recDiffOp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for different operator on simulate, got %d: %s", recDiffOp.Code, recDiffOp.Body.String())
	}

	// 4. Request with owning operator token: returns 200
	ownerOp := "op-owner-1"
	ownerToken := createTestToken(t, jwtSecret, "user-op-1", "operator_admin", &ownerOp)
	reqOwner := httptest.NewRequest(http.MethodPost, "/trips/sched-1/simulate", bytes.NewReader(bodyBytes))
	reqOwner.Header.Set("Authorization", "Bearer "+ownerToken)
	recOwner := httptest.NewRecorder()
	router.ServeHTTP(recOwner, reqOwner)

	if recOwner.Code != http.StatusOK {
		t.Fatalf("expected 200 for schedule owner on simulate, got %d: %s", recOwner.Code, recOwner.Body.String())
	}
}
