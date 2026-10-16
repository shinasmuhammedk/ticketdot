package repository

import (
	"context"

	"ticketdot/internal/payment/model"
)

type PaymentRepository interface {
	CreateOrder(ctx context.Context, p *model.Payment) error
	GetPaymentByOrderID(ctx context.Context, orderID string) (*model.Payment, error)
	GetPaymentByBookingRef(ctx context.Context, bookingRef string) (*model.Payment, error)
	VerifyAndCapture(ctx context.Context, userID string, req model.VerifyPaymentRequest, amount float64) (*model.VerifyPaymentResponse, error)
	RecordPaymentFailure(ctx context.Context, orderID, paymentID, errorCode, errorDesc string) error
	ProcessRefund(ctx context.Context, bookingRef, refundID string, refundAmount float64) error
	GetScheduleFare(ctx context.Context, scheduleID string) (float64, string, string, error)
	ValidateSeatLocks(ctx context.Context, scheduleID string, seats []string, userID string) error
}
