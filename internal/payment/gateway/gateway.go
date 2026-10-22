package gateway

import (
	"context"
	"errors"
)

var (
	ErrAmountTooLow = errors.New("amount must be at least 100 paise (1 Rupee)")
	ErrGatewayFailed = errors.New("payment gateway order creation failed")
)

type CreateOrderInput struct {
	AmountPaise int64             `json:"amount_paise"`
	Currency    string            `json:"currency"`
	Receipt     string            `json:"receipt"`
	Notes       map[string]string `json:"notes"`
}

type GatewayOrder struct {
	ID          string `json:"id"`
	AmountPaise int64  `json:"amount_paise"`
	Currency    string `json:"currency"`
	Receipt     string `json:"receipt"`
	Status      string `json:"status"`
}

type GatewayPayment struct {
	ID          string `json:"id"`
	OrderID     string `json:"order_id"`
	AmountPaise int64  `json:"amount_paise"`
	Currency    string `json:"currency"`
	Status      string `json:"status"` // "created", "authorized", "captured", "failed"
	Method      string `json:"method"` // "card", "upi", "netbanking", etc.
}

type Gateway interface {
	CreateOrder(ctx context.Context, in CreateOrderInput) (*GatewayOrder, error)
	FetchPayment(ctx context.Context, paymentID string) (*GatewayPayment, error)
	Name() string
}
