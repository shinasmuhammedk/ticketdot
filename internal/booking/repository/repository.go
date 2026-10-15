package repository

import "ticketdot/internal/booking/model"

type BookingRepository interface {
	LockSeats(userID, scheduleID string, seats []string) (*model.LockSeatResponse, error)
	ConfirmBooking(userID, scheduleID string, seats []string) (*model.ConfirmBookingResponse, error)
	ReleaseSeats(userID, scheduleID string, seats []string) (*model.ReleaseSeatsResponse, error)
	GetMyBookings(userID string) ([]model.MyBooking, error)
	CancelBooking(userID, bookingRef string) (*model.CancelBookingResponse, error)
	GetBookingDetails(userID, bookingRef string) (*model.BookingDetailsResponse, error)
}
