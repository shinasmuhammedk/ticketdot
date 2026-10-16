package model

import "time"

// Review represents a passenger review in the database
type Review struct {
	ID                string     `db:"id" json:"id"`
	BookingID         string     `db:"booking_id" json:"booking_id"`
	UserID            string     `db:"user_id" json:"user_id"`
	UserName          string     `db:"user_name" json:"user_name"`
	BusID             string     `db:"bus_id" json:"bus_id"`
	BusType           string     `db:"bus_type" json:"bus_type"`
	OperatorID        string     `db:"operator_id" json:"operator_id"`
	OperatorName      string     `db:"operator_name" json:"operator_name"`
	ScheduleID        string     `db:"schedule_id" json:"schedule_id"`
	BookingRef        string     `db:"booking_ref" json:"booking_ref"`
	Origin            string     `db:"origin" json:"origin"`
	Destination       string     `db:"destination" json:"destination"`
	Rating            float64    `db:"rating" json:"rating"`
	CleanlinessRating int        `db:"cleanliness_rating" json:"cleanliness_rating"`
	PunctualityRating int        `db:"punctuality_rating" json:"punctuality_rating"`
	ComfortRating     int        `db:"comfort_rating" json:"comfort_rating"`
	StaffRating       int        `db:"staff_rating" json:"staff_rating"`
	Headline          string     `db:"headline" json:"headline"`
	Comment           string     `db:"comment" json:"comment"`
	OperatorReply     *string    `db:"operator_reply" json:"operator_reply,omitempty"`
	OperatorRepliedAt *time.Time `db:"operator_replied_at" json:"operator_replied_at,omitempty"`
	CreatedAt         time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at" json:"updated_at"`
}

// SubmitReviewRequest payload sent by passenger
type SubmitReviewRequest struct {
	BookingRef        string `json:"booking_ref"`
	CleanlinessRating int    `json:"cleanliness_rating"`
	PunctualityRating int    `json:"punctuality_rating"`
	ComfortRating     int    `json:"comfort_rating"`
	StaffRating       int    `json:"staff_rating"`
	Headline          string `json:"headline"`
	Comment           string `json:"comment"`
}

// OperatorReplyRequest payload sent by operator
type OperatorReplyRequest struct {
	Reply string `json:"reply"`
}

// BusRatingSummary encapsulates aggregated ratings across all categories
type BusRatingSummary struct {
	BusID              string         `json:"bus_id"`
	AverageRating      float64        `json:"average_rating"`
	TotalReviews       int            `json:"total_reviews"`
	CleanlinessAvg     float64        `json:"cleanliness_avg"`
	PunctualityAvg     float64        `json:"punctuality_avg"`
	ComfortAvg         float64        `json:"comfort_avg"`
	StaffAvg           float64        `json:"staff_avg"`
	RatingDistribution map[string]int `json:"rating_distribution"`
}

// BookingTripDetails for validation before review submission
type BookingTripDetails struct {
	BookingID  string `db:"booking_id"`
	UserID     string `db:"user_id"`
	BusID      string `db:"bus_id"`
	OperatorID string `db:"operator_id"`
	ScheduleID string `db:"schedule_id"`
	Status     string `db:"status"`
}
