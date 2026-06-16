package model

import "time"

type LockSeatRequest struct {
	ScheduleID string `json:"schedule_id"`
	Seat       string `json:"seat"`
}

type LockSeatResponse struct {
	ScheduleID  string `json:"schedule_id"`
	Seat        string `json:"seat"`
	LockExpires string `json:"lock_expires"`
}

type ConfirmBookingRequest struct {
	ScheduleID string `json:"schedule_id"`
	Seat       string `json:"seat"`
}

type ConfirmBookingResponse struct {
	BookingRef string `json:"booking_ref"`
	ScheduleID string `json:"schedule_id"`
	Seat       string `json:"seat"`
	Status     string `json:"status"`
}

type MyBooking struct {
	BookingRef  string    `db:"booking_ref" json:"booking_ref"`
	ScheduleID  string    `db:"schedule_id" json:"schedule_id"`
	SeatNumber  string    `db:"seat_number" json:"seat_number"`
	Status      string    `db:"status" json:"status"`
	Amount      float64   `db:"amount" json:"amount"`
	ConfirmedAt time.Time `db:"confirmed_at" json:"confirmed_at"`
}

type CancelBookingResponse struct {
	BookingRef string `json:"booking_ref"`
	Status     string `json:"status"`
	Message    string `json:"message"`
}