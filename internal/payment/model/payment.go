package model

import (
	"time"

	"github.com/lib/pq"
)

type Payment struct {
	ID                string         `db:"id" json:"id"`
	OrderID           string         `db:"order_id" json:"order_id"`
	PaymentID         *string        `db:"payment_id" json:"payment_id,omitempty"`
	BookingRef        *string        `db:"booking_ref" json:"booking_ref,omitempty"`
	UserID            string         `db:"user_id" json:"user_id"`
	ScheduleID        string         `db:"schedule_id" json:"schedule_id"`
	Seats             pq.StringArray `db:"seats" json:"seats"`
	Amount            int64          `db:"amount" json:"amount"`                     // Amount in paise
	AmountPaise       int64          `db:"-" json:"amount_paise"`                   // Explicit paise field
	Currency          string         `db:"currency" json:"currency"`
	Status            string         `db:"status" json:"status"` // 'pending', 'captured', 'failed', 'refunded', 'expired', 'refund_required'
	PaymentMethod     *string        `db:"payment_method" json:"payment_method,omitempty"`
	Gateway           string         `db:"gateway" json:"gateway"`
	GatewaySignature  *string        `db:"gateway_signature" json:"gateway_signature,omitempty"`
	RefundID          *string        `db:"refund_id" json:"refund_id,omitempty"`
	RefundAmount      int64          `db:"refund_amount" json:"refund_amount"`       // Refund in paise
	RefundAmountPaise int64          `db:"-" json:"refund_amount_paise"`             // Explicit paise field
	RefundStatus      *string        `db:"refund_status" json:"refund_status,omitempty"`
	RefundedAt        *time.Time     `db:"refunded_at" json:"refunded_at,omitempty"`
	ErrorCode         *string        `db:"error_code" json:"error_code,omitempty"`
	ErrorDescription  *string        `db:"error_description" json:"error_description,omitempty"`
	GatewayPaymentID  *string        `db:"gateway_payment_id" json:"gateway_payment_id,omitempty"`
	LastErrorCode     *string        `db:"last_error_code" json:"last_error_code,omitempty"`
	ExpiresAt         *time.Time     `db:"expires_at" json:"expires_at,omitempty"`
	TicketEmailSentAt *time.Time     `db:"ticket_email_sent_at" json:"ticket_email_sent_at,omitempty"`
	CreatedAt         time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time      `db:"updated_at" json:"updated_at"`
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
	OrderID     string   `json:"order_id"`
	KeyID       string   `json:"key_id"`
	Amount      int64    `json:"amount"`       // Amount in paise
	AmountPaise int64    `json:"amount_paise"` // Explicit paise field
	Currency    string   `json:"currency"`
	ScheduleID  string   `json:"schedule_id"`
	Seats       []string `json:"seats"`
	CreatedAt   string   `json:"created_at"`
	ExpiresAt   string   `json:"expires_at"`
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
	BookingRef       string            `json:"booking_ref"`
	PaymentID        string            `json:"payment_id"`
	OrderID          string            `json:"order_id"`
	Status           string            `json:"status"` // 'confirmed', 'processing', 'refund_required'
	TotalAmount      int64             `json:"total_amount"`       // Amount in paise
	TotalAmountPaise int64             `json:"total_amount_paise"` // Explicit paise field
	Currency         string            `json:"currency"`
	PaymentMethod    string            `json:"payment_method"`
	ScheduleID       string            `json:"schedule_id"`
	Seats            []string          `json:"seats"`
	Passengers       []PassengerDetail `json:"passengers"`
	BoardingPoint    string            `json:"boarding_point"`
	DroppingPoint    string            `json:"dropping_point"`
	ContactPhone     string            `json:"contact_phone"`
	ContactEmail     string            `json:"contact_email"`
	ConfirmedAt      string            `json:"confirmed_at"`
	RefundRequired   bool              `json:"refund_required,omitempty"`
	Message          string            `json:"message,omitempty"`
}

type PaymentStatusResponse struct {
	OrderID           string     `json:"order_id"`
	PaymentID         *string    `json:"payment_id,omitempty"`
	GatewayPaymentID  *string    `json:"gateway_payment_id,omitempty"`
	BookingRef        *string    `json:"booking_ref,omitempty"`
	Status            string     `json:"status"`
	Amount            int64      `json:"amount"`                    // Amount in paise
	AmountPaise       int64      `json:"amount_paise"`              // Explicit paise field
	Currency          string     `json:"currency"`
	PaymentMethod     *string    `json:"payment_method,omitempty"`
	RefundID          *string    `json:"refund_id,omitempty"`
	RefundAmount      int64      `json:"refund_amount,omitempty"`   // Refund in paise
	RefundAmountPaise int64      `json:"refund_amount_paise,omitempty"` // Explicit paise field
	RefundStatus      *string    `json:"refund_status,omitempty"`
	RefundedAt        *time.Time `json:"refunded_at,omitempty"`
	RefundPending     bool       `json:"refund_pending"`
	CreatedAt         time.Time  `json:"created_at"`
}

type WebhookPayload struct {
	EventID string `json:"event_id,omitempty"`
	Event   string `json:"event"`
	Payload struct {
		Payment struct {
			Entity struct {
				ID          string `json:"id"`
				OrderID     string `json:"order_id"`
				Amount      int64  `json:"amount"` // Amount in paise
				Currency    string `json:"currency"`
				Status      string `json:"status"`
				Method      string `json:"method"`
				Description string `json:"description"`
				ErrorCode   string `json:"error_code"`
				ErrorDesc   string `json:"error_description"`
			} `json:"entity"`
		} `json:"payment"`
		Order struct {
			Entity struct {
				ID       string `json:"id"`
				Amount   int64  `json:"amount"` // Amount in paise
				Currency string `json:"currency"`
				Status   string `json:"status"`
			} `json:"entity"`
		} `json:"order"`
		Refund struct {
			Entity struct {
				ID        string `json:"id"`
				PaymentID string `json:"payment_id"`
				Amount    int64  `json:"amount"` // Amount in paise
				Status    string `json:"status"`
			} `json:"entity"`
		} `json:"refund"`
	} `json:"payload"`
}

type ConfirmationStatus string

const (
	ConfirmationStatusConfirmed        ConfirmationStatus = "confirmed"
	ConfirmationStatusAlreadyConfirmed ConfirmationStatus = "already_confirmed"
	ConfirmationStatusNotBookable      ConfirmationStatus = "not_bookable" // refund_required
)

type ConfirmationResult struct {
	Status            ConfirmationStatus     `json:"status"`
	Booking           *VerifyPaymentResponse `json:"booking,omitempty"`
	NotBookableReason string                 `json:"not_bookable_reason,omitempty"`
	Payment           *Payment               `json:"payment,omitempty"`
}

type ConfirmPaymentInput struct {
	OrderID          string            `json:"order_id"`
	UserID           string            `json:"user_id,omitempty"`
	PaymentID        string            `json:"payment_id"`
	GatewayPaymentID string            `json:"gateway_payment_id"`
	AmountPaise      int64             `json:"amount_paise"`
	Currency         string            `json:"currency"`
	PaymentMethod    string            `json:"payment_method"`
	Signature        string            `json:"signature"`
	Passengers       []PassengerDetail `json:"passengers"`
	BoardingPoint    string            `json:"boarding_point"`
	DroppingPoint    string            `json:"dropping_point"`
	ContactPhone     string            `json:"contact_phone"`
	ContactEmail     string            `json:"contact_email"`
}

type PaymentEvent struct {
	ID          int64      `db:"id" json:"id"`
	EventID     string     `db:"event_id" json:"event_id"`
	EventType   string     `db:"event_type" json:"event_type"`
	OrderID     *string    `db:"order_id" json:"order_id,omitempty"`
	PaymentID   *string    `db:"payment_id" json:"payment_id,omitempty"`
	Status      string     `db:"status" json:"status"` // 'processed', 'ignored', 'failed'
	ErrorCode   *string    `db:"error_code" json:"error_code,omitempty"`
	ReceivedAt  time.Time  `db:"received_at" json:"received_at"`
	ProcessedAt *time.Time `db:"processed_at" json:"processed_at,omitempty"`
}
