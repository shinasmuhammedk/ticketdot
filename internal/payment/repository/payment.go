package repository

import (
	"context"

	"github.com/jmoiron/sqlx"

	"ticketdot/internal/payment/model"
)

type PaymentRepository interface {
	CreateOrder(ctx context.Context, p *model.Payment) error
	CreateOrderAndExtendLocks(ctx context.Context, p *model.Payment, windowMinutes int) error
	FindPendingOrderByScheduleAndSeats(ctx context.Context, scheduleID string, seats []string) (*model.Payment, error)
	ExpirePaymentOrder(ctx context.Context, id string) error
	GetPaymentByOrderID(ctx context.Context, orderID string) (*model.Payment, error)
	GetPaymentByBookingRef(ctx context.Context, bookingRef string) (*model.Payment, error)
	VerifyAndCapture(ctx context.Context, userID string, req model.VerifyPaymentRequest, amount int64) (*model.VerifyPaymentResponse, error)
	ConfirmCapturedPayment(ctx context.Context, in model.ConfirmPaymentInput) (*model.ConfirmationResult, error)
	ConfirmCapturedPaymentInTx(ctx context.Context, tx *sqlx.Tx, in model.ConfirmPaymentInput) (*model.ConfirmationResult, error)
	RecordPaymentFailure(ctx context.Context, orderID, paymentID, userID, errorCode, errorDesc string) error
	RecordPaymentLastError(ctx context.Context, orderID, errorCode string) error
	RecordPaymentLastErrorInTx(ctx context.Context, tx *sqlx.Tx, orderID, errorCode string) error
	ExpireStalePendingOrders(ctx context.Context) (int64, error)
	ProcessRefund(ctx context.Context, bookingRef, refundID string, refundAmount int64) error
	GetScheduleFare(ctx context.Context, scheduleID string) (int64, string, string, error)
	ValidateSeatLocks(ctx context.Context, scheduleID string, seats []string, userID string) error
	ClaimTicketEmailDispatch(ctx context.Context, orderID string) (bool, error)
	RecordPaymentEventInTx(ctx context.Context, tx *sqlx.Tx, eventID, eventType, orderID, paymentID, status, errorCode string) (bool, error)
	UpdatePaymentEventInTx(ctx context.Context, tx *sqlx.Tx, eventID, status, errorCode, orderID, paymentID string) error
	GetPaymentEventByEventID(ctx context.Context, eventID string) (*model.PaymentEvent, error)
	ListRefundRequiredPayments(ctx context.Context) ([]model.Payment, error)
	BeginTx(ctx context.Context) (*sqlx.Tx, error)
}
