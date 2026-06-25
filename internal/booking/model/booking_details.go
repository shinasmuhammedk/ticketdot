package model

type BookingDetailsResponse struct {
	BookingRef string  `db:"booking_ref" json:"booking_ref"`
	ScheduleID string  `db:"schedule_id" json:"schedule_id"`
	SeatNumber string  `db:"seat_number" json:"seat_number"`
	Status     string  `db:"status" json:"status"`
	Amount     float64 `db:"amount" json:"amount"`
}