package model

import "time"

type LockSeatRequest struct {
	ScheduleID string   `json:"schedule_id"`
	Seat       string   `json:"seat,omitempty"`
	Seats      []string `json:"seats,omitempty"`
}

type LockSeatResponse struct {
	ScheduleID  string   `json:"schedule_id"`
	Seat        string   `json:"seat,omitempty"`
	Seats       []string `json:"seats"`
	LockExpires string   `json:"lock_expires"`
}

type PassengerDetail struct {
	SeatNumber string `json:"seat_number"`
	Name       string `json:"name"`
	Age        int    `json:"age"`
	Gender     string `json:"gender"`
}

type ConfirmBookingRequest struct {
	ScheduleID    string            `json:"schedule_id"`
	Seat          string            `json:"seat,omitempty"`
	Seats         []string          `json:"seats,omitempty"`
	Passengers    []PassengerDetail `json:"passengers,omitempty"`
	BoardingPoint string            `json:"boarding_point,omitempty"`
	DroppingPoint string            `json:"dropping_point,omitempty"`
	ContactPhone  string            `json:"contact_phone,omitempty"`
	ContactEmail  string            `json:"contact_email,omitempty"`
}

type ConfirmBookingResponse struct {
	BookingRef    string            `json:"booking_ref"`
	ScheduleID    string            `json:"schedule_id"`
	Seat          string            `json:"seat,omitempty"`
	Seats         []string          `json:"seats"`
	Passengers    []PassengerDetail `json:"passengers,omitempty"`
	BoardingPoint string            `json:"boarding_point,omitempty"`
	DroppingPoint string            `json:"dropping_point,omitempty"`
	ContactPhone  string            `json:"contact_phone,omitempty"`
	ContactEmail  string            `json:"contact_email,omitempty"`
	Status        string            `json:"status"`
	TotalAmount   float64           `json:"total_amount,omitempty"`
}

type ReleaseSeatsRequest struct {
	ScheduleID string   `json:"schedule_id"`
	Seat       string   `json:"seat,omitempty"`
	Seats      []string `json:"seats,omitempty"`
}

type ReleaseSeatsResponse struct {
	Message string   `json:"message"`
	Seats   []string `json:"seats"`
}

type MyBooking struct {
	BookingRef    string    `db:"booking_ref" json:"booking_ref"`
	ScheduleID    string    `db:"schedule_id" json:"schedule_id"`
	SeatNumber    string    `db:"seat_number" json:"seat_number,omitempty"`
	Seat          string    `json:"seat,omitempty"`
	Seats         []string  `json:"seats"`
	Origin        string    `db:"origin" json:"origin"`
	Destination   string    `db:"destination" json:"destination"`
	OperatorName  string    `db:"operator_name" json:"operator_name"`
	BusNumber     string    `db:"bus_number" json:"bus_number"`
	BusType       string    `db:"bus_type" json:"bus_type"`
	DepartureTime string    `db:"departure_time" json:"departure_time"`
	ArrivalTime   string    `db:"arrival_time" json:"arrival_time"`
	Status        string    `db:"status" json:"status"`
	Amount        float64   `db:"amount" json:"amount"`
	ConfirmedAt   time.Time `db:"confirmed_at" json:"confirmed_at"`
	PaymentID     string    `db:"payment_id" json:"payment_id,omitempty"`
	PaymentStatus string    `db:"payment_status" json:"payment_status,omitempty"`
	RefundID      string    `db:"refund_id" json:"refund_id,omitempty"`
	RefundStatus  string    `db:"refund_status" json:"refund_status,omitempty"`
}

type CancelBookingResponse struct {
	BookingRef     string   `json:"booking_ref"`
	Status         string   `json:"status"`
	Message        string   `json:"message"`
	Seats          []string `json:"seats,omitempty"`
	RefundID       string   `json:"refund_id,omitempty"`
	RefundAmount   float64  `json:"refund_amount,omitempty"`
	RefundStatus   string   `json:"refund_status,omitempty"`
	RefundTimeline string   `json:"refund_timeline,omitempty"`
}
