package model

type CreateScheduleRequest struct {
	RouteID        string  `json:"route_id"`
	BusID          string  `json:"bus_id"`
	DepartureTime  string  `json:"departure_time"`
	ArrivalTime    string  `json:"arrival_time"`
	AvailableSeats int     `json:"available_seats"`
	BaseFare       float64 `json:"base_fare"`
}

type CreateScheduleResponse struct {
	ID             string  `json:"id"`
	RouteID        string  `json:"route_id"`
	BusID          string  `json:"bus_id"`
	OperatorID     string  `json:"operator_id"`
	DepartureTime  string  `json:"departure_time"`
	ArrivalTime    string  `json:"arrival_time"`
	AvailableSeats int     `json:"available_seats"`
	BaseFare       float64 `json:"base_fare"`
	Status         string  `json:"status"`
}

type ScheduleResponse struct {
	ID string `db:"id" json:"id"`

	Origin      string `db:"origin" json:"origin"`
	Destination string `db:"destination" json:"destination"`

	BusNumber string `db:"bus_number" json:"bus_number"`
	BusType   string `db:"bus_type" json:"bus_type"`

	DepartureTime string `db:"departure_time" json:"departure_time"`
	ArrivalTime   string `db:"arrival_time" json:"arrival_time"`

	AvailableSeats int     `db:"available_seats" json:"available_seats"`
	BaseFare       float64 `db:"base_fare" json:"base_fare"`

	Status string `db:"status" json:"status"`
}
