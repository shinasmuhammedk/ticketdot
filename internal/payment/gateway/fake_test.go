package gateway_test

import (
	"context"
	"strings"
	"testing"

	"ticketdot/internal/payment/gateway"
)

func TestFakeGatewayCreateOrder(t *testing.T) {
	gw := gateway.NewFakeGateway()

	t.Run("creates fake order with order_fake_ prefix", func(t *testing.T) {
		in := gateway.CreateOrderInput{
			AmountPaise: 50000,
			Currency:    "INR",
			Receipt:     "rcpt_12345",
			Notes:       map[string]string{"foo": "bar"},
		}

		order, err := gw.CreateOrder(context.Background(), in)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.HasPrefix(order.ID, "order_fake_") {
			t.Fatalf("expected order ID to start with order_fake_, got %s", order.ID)
		}
		if order.AmountPaise != 50000 {
			t.Fatalf("expected amount %d, got %d", 50000, order.AmountPaise)
		}
		if order.Currency != "INR" {
			t.Fatalf("expected currency INR, got %s", order.Currency)
		}
		if order.Receipt != "rcpt_12345" {
			t.Fatalf("expected receipt rcpt_12345, got %s", order.Receipt)
		}
		if order.Status != "created" {
			t.Fatalf("expected status created, got %s", order.Status)
		}
	})

	t.Run("rejects amount < 100 paise", func(t *testing.T) {
		in := gateway.CreateOrderInput{
			AmountPaise: 99,
			Currency:    "INR",
		}

		_, err := gw.CreateOrder(context.Background(), in)
		if err == nil {
			t.Fatalf("expected error for amount < 100 paise, got nil")
		}
	})
}
