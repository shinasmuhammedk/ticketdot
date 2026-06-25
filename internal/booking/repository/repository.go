package repository

import "ticketdot/internal/booking/model"

type BookingRepository interface {
	LockSeat(userID, scheduleID, seat string) (*model.LockSeatResponse, error)
	ConfirmBooking(userID, scheduleID, seat string) (*model.ConfirmBookingResponse, error)
    GetMyBookings(userID string) ([]model.MyBooking, error)
    CancelBooking(userID, bookingRef string) (*model.CancelBookingResponse, error)
    GetBookingDetails(userID, bookingRef string) (*model.BookingDetailsResponse, error)
}
