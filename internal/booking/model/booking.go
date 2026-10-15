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

type ConfirmBookingRequest struct {
	ScheduleID string   `json:"schedule_id"`
	Seat       string   `json:"seat,omitempty"`
	Seats      []string `json:"seats,omitempty"`
}

type ConfirmBookingResponse struct {
	BookingRef  string   `json:"booking_ref"`
	ScheduleID  string   `json:"schedule_id"`
	Seat        string   `json:"seat,omitempty"`
	Seats       []string `json:"seats"`
	Status      string   `json:"status"`
	TotalAmount float64  `json:"total_amount,omitempty"`
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
	BookingRef  string    `db:"booking_ref" json:"booking_ref"`
	ScheduleID  string    `db:"schedule_id" json:"schedule_id"`
	SeatNumber  string    `db:"seat_number" json:"seat_number,omitempty"`
	Seat        string    `json:"seat,omitempty"`
	Seats       []string  `json:"seats"`
	Status      string    `db:"status" json:"status"`
	Amount      float64   `db:"amount" json:"amount"`
	ConfirmedAt time.Time `db:"confirmed_at" json:"confirmed_at"`
}

type CancelBookingResponse struct {
	BookingRef string   `json:"booking_ref"`
	Status     string   `json:"status"`
	Message    string   `json:"message"`
	Seats      []string `json:"seats,omitempty"`
}
