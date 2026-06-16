package model

import "time"

type BusSearchResult struct {
	ScheduleID      string    `db:"schedule_id" json:"schedule_id"`
	OperatorName    string    `db:"operator_name" json:"operator_name"`
	BusID           string    `db:"bus_id" json:"bus_id"`
	BusType         string    `db:"bus_type" json:"bus_type"`
	DepartureTime   time.Time `db:"departure_time" json:"departure_time"`
	ArrivalTime     time.Time `db:"arrival_time" json:"arrival_time"`
	AvailableSeats  int       `db:"available_seats" json:"available_seats"`
	BaseFare        float64   `db:"base_fare" json:"base_fare"`
}