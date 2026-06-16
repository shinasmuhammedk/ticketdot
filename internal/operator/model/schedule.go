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