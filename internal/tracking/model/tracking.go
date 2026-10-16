package model

import "time"

// TripTracking represents the real-time tracking row in PostgreSQL
type TripTracking struct {
	ID               string     `db:"id" json:"id"`
	ScheduleID       string     `db:"schedule_id" json:"schedule_id"`
	CurrentLatitude  float64    `db:"current_latitude" json:"current_latitude"`
	CurrentLongitude float64    `db:"current_longitude" json:"current_longitude"`
	SpeedKmh         float64    `db:"speed_kmh" json:"speed_kmh"`
	HeadingDegrees   float64    `db:"heading_degrees" json:"heading_degrees"`
	TripStatus       string     `db:"trip_status" json:"trip_status"` // scheduled, boarding, in_transit, delayed, completed
	DelayMinutes     int        `db:"delay_minutes" json:"delay_minutes"`
	CurrentStopName  *string    `db:"current_stop_name" json:"current_stop_name,omitempty"`
	NextStopName     *string    `db:"next_stop_name" json:"next_stop_name,omitempty"`
	NextStopETA      *time.Time `db:"next_stop_eta" json:"next_stop_eta,omitempty"`
	ProgressPercent  float64    `db:"progress_percent" json:"progress_percent"`
	LastUpdatedAt    time.Time  `db:"last_updated_at" json:"last_updated_at"`
}

// ScheduleRouteInfo encapsulates joined query information for a trip
type ScheduleRouteInfo struct {
	ScheduleID     string    `db:"schedule_id"`
	OperatorID     string    `db:"operator_id"`
	OperatorName   string    `db:"operator_name"`
	BusID          string    `db:"bus_id"`
	BusType        string    `db:"bus_type"`
	RegistrationNo string    `db:"registration_no"`
	DepartureTime  time.Time `db:"departure_time"`
	ArrivalTime    time.Time `db:"arrival_time"`
	OriginName     string    `db:"origin_name"`
	OriginLat      float64   `db:"origin_lat"`
	OriginLng      float64   `db:"origin_lng"`
	DestName       string    `db:"dest_name"`
	DestLat        float64   `db:"dest_lat"`
	DestLng        float64   `db:"dest_lng"`
	ScheduleStatus string    `db:"schedule_status"`
}

// Waypoint represents a geocoordinate on the route polyline
type Waypoint struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// TripStop represents a physical milestone or pickup point along the route
type TripStop struct {
	Name          string  `json:"name"`
	City          string  `json:"city"`
	Lat           float64 `json:"lat"`
	Lng           float64 `json:"lng"`
	EstimatedTime string  `json:"estimated_time"`
	Status        string  `json:"status"` // "passed", "current", "upcoming"
	IsBoarding    bool    `json:"is_boarding"`
	IsDropping    bool    `json:"is_dropping"`
}

// TripTrackingDetails is the full consumer payload delivered to frontend tracking UI
type TripTrackingDetails struct {
	ScheduleID       string     `json:"schedule_id"`
	OperatorName     string     `json:"operator_name"`
	BusType          string     `json:"bus_type"`
	RegistrationNo   string     `json:"registration_no"`
	Origin           string     `json:"origin"`
	Destination      string     `json:"destination"`
	OriginCoords     Waypoint   `json:"origin_coords"`
	DestCoords       Waypoint   `json:"dest_coords"`
	DepartureTime    time.Time  `json:"departure_time"`
	ArrivalTime      time.Time  `json:"arrival_time"`
	TripStatus       string     `json:"trip_status"`
	CurrentLatitude  float64    `json:"current_latitude"`
	CurrentLongitude float64    `json:"current_longitude"`
	SpeedKmh         float64    `json:"speed_kmh"`
	HeadingDegrees   float64    `json:"heading_degrees"`
	DelayMinutes     int        `json:"delay_minutes"`
	ProgressPercent  float64    `json:"progress_percent"`
	CurrentStopName  string     `json:"current_stop_name"`
	NextStopName     string     `json:"next_stop_name"`
	NextStopETA      *time.Time `json:"next_stop_eta,omitempty"`
	LastUpdatedAt    time.Time  `json:"last_updated_at"`
	Waypoints        []Waypoint `json:"waypoints"`
	Stops            []TripStop `json:"stops"`
}

// UpdateLocationRequest is sent by operator driver telemetry
type UpdateLocationRequest struct {
	Latitude        float64    `json:"latitude"`
	Longitude       float64    `json:"longitude"`
	SpeedKmh        float64    `json:"speed_kmh"`
	HeadingDegrees  float64    `json:"heading_degrees"`
	TripStatus      string     `json:"trip_status"`
	DelayMinutes    int        `json:"delay_minutes"`
	CurrentStopName string     `json:"current_stop_name"`
	NextStopName    string     `json:"next_stop_name"`
	NextStopETA     *time.Time `json:"next_stop_eta,omitempty"`
	ProgressPercent float64    `json:"progress_percent"`
}

// SimulateTripRequest is used by test benches & operator demo controls
type SimulateTripRequest struct {
	Action       string `json:"action"` // "start", "step", "delay", "complete", "reset"
	DelayMinutes int    `json:"delay_minutes,omitempty"`
}
