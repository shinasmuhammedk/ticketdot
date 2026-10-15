package model

type BookingDetailsResponse struct {
	BookingRef string `db:"booking_ref" json:"booking_ref"`

	PassengerName string `db:"passenger_name" json:"passenger_name"`
	OperatorName  string `db:"operator_name" json:"operator_name"`

	Origin      string `db:"origin" json:"origin"`
	Destination string `db:"destination" json:"destination"`

	BusNumber        string `db:"bus_number" json:"bus_number"`
	BusRegistration  string `json:"bus_registration,omitempty"`
	BusType          string `db:"bus_type" json:"bus_type"`

	SeatNumber string   `db:"seat_number" json:"seat_number"`
	Seats      []string `json:"seats"`

	DepartureTime string `db:"departure_time" json:"departure_time"`
	ArrivalTime   string `db:"arrival_time" json:"arrival_time"`

	Amount float64 `db:"amount" json:"amount"`
	Status string  `db:"status" json:"status"`
}

