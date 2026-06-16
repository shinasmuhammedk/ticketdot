package model

type CreateBusRequest struct {
	RegistrationNo string `json:"registration_no"`
	BusType         string `json:"bus_type"`
	TotalSeats      int    `json:"total_seats"`
}

type CreateBusResponse struct {
	ID             string `json:"id"`
	RegistrationNo string `json:"registration_no"`
	BusType         string `json:"bus_type"`
	TotalSeats      int    `json:"total_seats"`
}