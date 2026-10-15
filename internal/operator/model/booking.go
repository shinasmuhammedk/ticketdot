package model

import "time"

type OperatorBooking struct {
	BookingRef     string    `db:"booking_ref" json:"booking_ref"`
	ScheduleID     string    `db:"schedule_id" json:"schedule_id"`
	PassengerName  string    `db:"passenger_name" json:"passenger_name"`
	PassengerEmail string    `db:"passenger_email" json:"passenger_email"`
	PassengerPhone string    `db:"passenger_phone" json:"passenger_phone"`
	Origin         string    `db:"origin" json:"origin"`
	Destination    string    `db:"destination" json:"destination"`
	BusNumber      string    `db:"bus_number" json:"bus_number"`
	SeatNumber     string    `db:"seat_number" json:"seat_number"`
	Status         string    `db:"status" json:"status"`
	Amount         float64   `db:"amount" json:"amount"`
	ConfirmedAt    time.Time `db:"confirmed_at" json:"confirmed_at"`
}
