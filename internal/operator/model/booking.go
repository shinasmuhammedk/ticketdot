package model

import "time"

type OperatorBooking struct {
	BookingRef  string    `db:"booking_ref" json:"booking_ref"`
	ScheduleID  string    `db:"schedule_id" json:"schedule_id"`
	SeatNumber  string    `db:"seat_number" json:"seat_number"`
	Status      string    `db:"status" json:"status"`
	Amount      float64   `db:"amount" json:"amount"`
	ConfirmedAt time.Time `db:"confirmed_at" json:"confirmed_at"`
}