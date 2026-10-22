package model

import "time"

type BusSearchResult struct {
	ScheduleID     string    `db:"schedule_id" json:"schedule_id"`
	OperatorName   string    `db:"operator_name" json:"operator_name"`
	BusID          string    `db:"bus_id" json:"bus_id"`
	BusType        string    `db:"bus_type" json:"bus_type"`
	DepartureTime  time.Time `db:"departure_time" json:"departure_time"`
	ArrivalTime    time.Time `db:"arrival_time" json:"arrival_time"`
	AvailableSeats int       `db:"available_seats" json:"available_seats"`
	BaseFare       int64     `db:"base_fare" json:"base_fare"`       // Fare in paise
	BaseFarePaise  int64     `db:"-" json:"base_fare_paise"`         // Explicit paise field
	Rating         float64   `db:"rating" json:"rating"`
	ReviewCount    int       `db:"review_count" json:"review_count"`
}

type ScheduleDetailsResponse struct {
	ID            string    `db:"id" json:"id"`
	ScheduleID    string    `db:"schedule_id" json:"schedule_id"`
	Origin        string    `db:"origin" json:"origin"`
	Destination   string    `db:"destination" json:"destination"`
	OperatorName  string    `db:"operator_name" json:"operator_name"`
	BusType       string    `db:"bus_type" json:"bus_type"`
	DepartureTime time.Time `db:"departure_time" json:"departure_time"`
	ArrivalTime   time.Time `db:"arrival_time" json:"arrival_time"`
	BaseFare      int64     `db:"base_fare" json:"base_fare"`       // Fare in paise
	BaseFarePaise int64     `db:"-" json:"base_fare_paise"`         // Explicit paise field
}

type City struct {
	ID    string `db:"id" json:"id"`
	Name  string `db:"name" json:"name"`
	State string `db:"state" json:"state"`
}
