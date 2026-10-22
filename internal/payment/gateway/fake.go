package gateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"

	"ticketdot/internal/httpx"
)

type FakeGateway struct {
	mu       sync.RWMutex
	payments map[string]*GatewayPayment
}

func NewFakeGateway() *FakeGateway {
	return &FakeGateway{
		payments: make(map[string]*GatewayPayment),
	}
}

func (g *FakeGateway) Name() string {
	return "fake"
}

func (g *FakeGateway) RegisterPayment(paymentID, orderID string, amountPaise int64, currency, status, method string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "" {
		currency = "INR"
	}
	if status == "" {
		status = "captured"
	}
	if method == "" {
		method = "card"
	}

	g.payments[paymentID] = &GatewayPayment{
		ID:          paymentID,
		OrderID:     orderID,
		AmountPaise: amountPaise,
		Currency:    currency,
		Status:      status,
		Method:      method,
	}
}

func (g *FakeGateway) CreateOrder(ctx context.Context, in CreateOrderInput) (*GatewayOrder, error) {
	if in.AmountPaise < 100 {
		return nil, httpx.Validation("amount must be at least 100 paise (₹1.00)")
	}

	randomBytes := make([]byte, 8)
	_, _ = rand.Read(randomBytes)
	orderID := "order_fake_" + hex.EncodeToString(randomBytes)

	currency := strings.ToUpper(strings.TrimSpace(in.Currency))
	if currency == "" {
		currency = "INR"
	}

	return &GatewayOrder{
		ID:          orderID,
		AmountPaise: in.AmountPaise,
		Currency:    currency,
		Receipt:     in.Receipt,
		Status:      "created",
	}, nil
}

func (g *FakeGateway) FetchPayment(ctx context.Context, paymentID string) (*GatewayPayment, error) {
	paymentID = strings.TrimSpace(paymentID)
	if !paymentIDRegex.MatchString(paymentID) {
		return nil, httpx.Validation("invalid payment id format")
	}

	g.mu.RLock()
	defer g.mu.RUnlock()

	p, ok := g.payments[paymentID]
	if !ok {
		return nil, httpx.NotFound("payment not found on gateway")
	}

	cp := *p
	return &cp, nil
}
