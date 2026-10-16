package model

import (
	"time"

	"github.com/lib/pq"
)

type Payment struct {
	ID               string         `db:"id" json:"id"`
	OrderID          string         `db:"order_id" json:"order_id"`
	PaymentID        *string        `db:"payment_id" json:"payment_id,omitempty"`
	BookingRef       *string        `db:"booking_ref" json:"booking_ref,omitempty"`
	UserID           string         `db:"user_id" json:"user_id"`
	ScheduleID       string         `db:"schedule_id" json:"schedule_id"`
	Seats            pq.StringArray `db:"seats" json:"seats"`
	Amount           float64    `db:"amount" json:"amount"`
	Currency         string     `db:"currency" json:"currency"`
	Status           string     `db:"status" json:"status"` // 'pending', 'captured', 'failed', 'refunded'
	PaymentMethod    *string    `db:"payment_method" json:"payment_method,omitempty"`
	Gateway          string     `db:"gateway" json:"gateway"`
	GatewaySignature *string    `db:"gateway_signature" json:"gateway_signature,omitempty"`
	RefundID         *string    `db:"refund_id" json:"refund_id,omitempty"`
	RefundAmount     float64    `db:"refund_amount" json:"refund_amount"`
	RefundStatus     *string    `db:"refund_status" json:"refund_status,omitempty"`
	RefundedAt       *time.Time `db:"refunded_at" json:"refunded_at,omitempty"`
	ErrorCode        *string    `db:"error_code" json:"error_code,omitempty"`
	ErrorDescription *string    `db:"error_description" json:"error_description,omitempty"`
	CreatedAt        time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at" json:"updated_at"`
}

type PassengerDetail struct {
	SeatNumber string `json:"seat_number"`
	Name       string `json:"name"`
	Age        int    `json:"age"`
	Gender     string `json:"gender"`
}

type CreateOrderRequest struct {
	ScheduleID   string   `json:"schedule_id"`
	Seats        []string `json:"seats"`
	ContactPhone string   `json:"contact_phone,omitempty"`
	ContactEmail string   `json:"contact_email,omitempty"`
}

type CreateOrderResponse struct {
	OrderID    string   `json:"order_id"`
	ScheduleID string   `json:"schedule_id"`
	Seats      []string `json:"seats"`
	Amount     float64  `json:"amount"`
	Currency   string   `json:"currency"`
	KeyID      string   `json:"key_id"`
	CreatedAt  string   `json:"created_at"`
}

type VerifyPaymentRequest struct {
	OrderID       string            `json:"order_id"`
	PaymentID     string            `json:"payment_id"`
	Signature     string            `json:"signature"`
	PaymentMethod string            `json:"payment_method"`
	ScheduleID    string            `json:"schedule_id"`
	Seats         []string          `json:"seats"`
	Passengers    []PassengerDetail `json:"passengers"`
	BoardingPoint string            `json:"boarding_point,omitempty"`
	DroppingPoint string            `json:"dropping_point,omitempty"`
	ContactPhone  string            `json:"contact_phone,omitempty"`
	ContactEmail  string            `json:"contact_email,omitempty"`
}

type VerifyPaymentResponse struct {
	BookingRef    string            `json:"booking_ref"`
	PaymentID     string            `json:"payment_id"`
	OrderID       string            `json:"order_id"`
	Status        string            `json:"status"`
	TotalAmount   float64           `json:"total_amount"`
	Currency      string            `json:"currency"`
	PaymentMethod string            `json:"payment_method"`
	ScheduleID    string            `json:"schedule_id"`
	Seats         []string          `json:"seats"`
	Passengers    []PassengerDetail `json:"passengers"`
	BoardingPoint string            `json:"boarding_point"`
	DroppingPoint string            `json:"dropping_point"`
	ContactPhone  string            `json:"contact_phone"`
	ContactEmail  string            `json:"contact_email"`
	ConfirmedAt   string            `json:"confirmed_at"`
}

type PaymentStatusResponse struct {
	OrderID       string     `json:"order_id"`
	PaymentID     *string    `json:"payment_id,omitempty"`
	BookingRef    *string    `json:"booking_ref,omitempty"`
	Status        string     `json:"status"`
	Amount        float64    `json:"amount"`
	Currency      string     `json:"currency"`
	PaymentMethod *string    `json:"payment_method,omitempty"`
	RefundID      *string    `json:"refund_id,omitempty"`
	RefundAmount  float64    `json:"refund_amount,omitempty"`
	RefundStatus  *string    `json:"refund_status,omitempty"`
	RefundedAt    *time.Time `json:"refunded_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type WebhookPayload struct {
	Event   string `json:"event"`
	Payload struct {
		Payment struct {
			Entity struct {
				ID          string  `json:"id"`
				OrderID     string  `json:"order_id"`
				Amount      float64 `json:"amount"`
				Currency    string  `json:"currency"`
				Status      string  `json:"status"`
				Method      string  `json:"method"`
				Description string  `json:"description"`
				ErrorCode   string  `json:"error_code"`
				ErrorDesc   string  `json:"error_description"`
			} `json:"entity"`
		} `json:"payment"`
		Refund struct {
			Entity struct {
				ID        string  `json:"id"`
				PaymentID string  `json:"payment_id"`
				Amount    float64 `json:"amount"`
				Status    string  `json:"status"`
			} `json:"entity"`
		} `json:"refund"`
	} `json:"payload"`
}
