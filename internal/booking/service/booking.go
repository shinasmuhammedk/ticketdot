package service

import (
	"errors"
	"strings"

	"ticketdot/internal/booking/model"
	"ticketdot/internal/booking/repository"
)

type BookingService struct {
	repo repository.BookingRepository
}

func NewBookingService(repo repository.BookingRepository) *BookingService {
	return &BookingService{
		repo: repo,
	}
}

func (s *BookingService) LockSeat(
	userID string,
	req model.LockSeatRequest,
) (*model.LockSeatResponse, error) {

	userID = strings.TrimSpace(userID)
	req.ScheduleID = strings.TrimSpace(req.ScheduleID)
	req.Seat = strings.TrimSpace(strings.ToUpper(req.Seat))

	if userID == "" {
		return nil, errors.New("user id is required")
	}

	if req.ScheduleID == "" {
		return nil, errors.New("schedule id is required")
	}

	if req.Seat == "" {
		return nil, errors.New("seat is required")
	}

	return s.repo.LockSeat(
		userID,
		req.ScheduleID,
		req.Seat,
	)
}


func (s *BookingService) ConfirmBooking(
	userID string,
	req model.ConfirmBookingRequest,
) (*model.ConfirmBookingResponse, error) {

	userID = strings.TrimSpace(userID)
	req.ScheduleID = strings.TrimSpace(req.ScheduleID)
	req.Seat = strings.TrimSpace(strings.ToUpper(req.Seat))

	if userID == "" {
		return nil, errors.New("user id is required")
	}

	if req.ScheduleID == "" {
		return nil, errors.New("schedule id is required")
	}

	if req.Seat == "" {
		return nil, errors.New("seat is required")
	}

	return s.repo.ConfirmBooking(userID, req.ScheduleID, req.Seat)
}


func (s *BookingService) GetMyBookings(userID string) ([]model.MyBooking, error) {
	userID = strings.TrimSpace(userID)

	if userID == "" {
		return nil, errors.New("user id is required")
	}

	return s.repo.GetMyBookings(userID)
}


func (s *BookingService) CancelBooking(
	userID string,
	bookingRef string,
) (*model.CancelBookingResponse, error) {

	userID = strings.TrimSpace(userID)
	bookingRef = strings.TrimSpace(bookingRef)

	if userID == "" {
		return nil, errors.New("user id is required")
	}

	if bookingRef == "" {
		return nil, errors.New("booking reference is required")
	}

	return s.repo.CancelBooking(userID, bookingRef)
}