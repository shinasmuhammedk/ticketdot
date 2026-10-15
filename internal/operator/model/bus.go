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
