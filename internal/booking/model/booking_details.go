package model

type PassengerInfo struct {
	SeatNumber string `json:"seat_number"`
	Name       string `json:"name"`
	Age        int    `json:"age"`
	Gender     string `json:"gender"`
}

type BookingDetailsResponse struct {
	BookingRef string `db:"booking_ref" json:"booking_ref"`

	PassengerName string `db:"passenger_name" json:"passenger_name"`
	OperatorName  string `db:"operator_name" json:"operator_name"`

	Origin      string `db:"origin" json:"origin"`
	Destination string `db:"destination" json:"destination"`

	BoardingPoint string `db:"boarding_point" json:"boarding_point"`
	DroppingPoint string `db:"dropping_point" json:"dropping_point"`
	ContactPhone  string `db:"contact_phone" json:"contact_phone"`
	ContactEmail  string `db:"contact_email" json:"contact_email"`

	BusNumber       string `db:"bus_number" json:"bus_number"`
	BusRegistration string `json:"bus_registration,omitempty"`
	BusType         string `db:"bus_type" json:"bus_type"`

	SeatNumber string          `db:"seat_number" json:"seat_number"`
	Seats      []string        `json:"seats"`
	Passengers []PassengerInfo `json:"passengers"`

	DepartureTime string `db:"departure_time" json:"departure_time"`
	ArrivalTime   string `db:"arrival_time" json:"arrival_time"`

	Amount float64 `db:"amount" json:"amount"`
	Status string  `db:"status" json:"status"`

	// Payment & Refund tracking
	PaymentID     string  `json:"payment_id,omitempty"`
	PaymentMethod string  `json:"payment_method,omitempty"`
	PaymentStatus string  `json:"payment_status,omitempty"`
	RefundID      string  `json:"refund_id,omitempty"`
	RefundAmount  float64 `json:"refund_amount,omitempty"`
	RefundStatus  string  `json:"refund_status,omitempty"`
}
