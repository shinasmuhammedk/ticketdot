package model

type CreateBusRequest struct {
	RegistrationNo string `json:"registration_no"`
	BusType        string `json:"bus_type"`
	TotalSeats     int    `json:"total_seats"`
}

type CreateBusResponse struct {
	ID             string `db:"id" json:"id"`
	RegistrationNo string `db:"registration_no" json:"registration_no"`
	BusType        string `db:"bus_type" json:"bus_type"`
	TotalSeats     int    `db:"total_seats" json:"total_seats"`
}

type UpdateBusRequest struct {
	RegistrationNo string `json:"registration_no"`
	BusType        string `json:"bus_type"`
	TotalSeats     int    `json:"total_seats"`
}

type BusSeatLayout struct {
	SeatNumber string `db:"seat_number" json:"seat_number"`
	SeatType   string `db:"seat_type" json:"seat_type"`
	DeckType   string `db:"deck_type" json:"deck_type"`
	RowNum     int    `db:"row_num" json:"row_num"`
	ColNum     int    `db:"col_num" json:"col_num"`
	IsWindow   bool   `db:"is_window" json:"is_window"`
}
