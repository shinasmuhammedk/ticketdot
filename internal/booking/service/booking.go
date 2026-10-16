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

func extractSeats(single string, multiple []string) []string {
	var seats []string
	if len(multiple) > 0 {
		seats = append(seats, multiple...)
	} else if strings.TrimSpace(single) != "" {
		seats = append(seats, strings.TrimSpace(single))
	}
	return seats
}

func (s *BookingService) LockSeat(
	userID string,
	req model.LockSeatRequest,
) (*model.LockSeatResponse, error) {

	userID = strings.TrimSpace(userID)
	req.ScheduleID = strings.TrimSpace(req.ScheduleID)
	seats := extractSeats(req.Seat, req.Seats)

	if userID == "" {
		return nil, errors.New("user id is required")
	}

	if req.ScheduleID == "" {
		return nil, errors.New("schedule id is required")
	}

	if len(seats) == 0 {
		return nil, errors.New("at least one seat is required")
	}

	return s.repo.LockSeats(
		userID,
		req.ScheduleID,
		seats,
	)
}

func (s *BookingService) ConfirmBooking(
	userID string,
	req model.ConfirmBookingRequest,
) (*model.ConfirmBookingResponse, error) {

	userID = strings.TrimSpace(userID)
	req.ScheduleID = strings.TrimSpace(req.ScheduleID)
	seats := extractSeats(req.Seat, req.Seats)

	// If seats are empty but passengers are provided, extract seat numbers from passengers
	if len(seats) == 0 && len(req.Passengers) > 0 {
		for _, p := range req.Passengers {
			if seat := strings.TrimSpace(p.SeatNumber); seat != "" {
				seats = append(seats, seat)
			}
		}
	}

	if userID == "" {
		return nil, errors.New("user id is required")
	}

	if req.ScheduleID == "" {
		return nil, errors.New("schedule id is required")
	}

	if len(seats) == 0 {
		return nil, errors.New("at least one seat is required")
	}

	req.Seats = seats

	return s.repo.ConfirmBooking(userID, req)
}


func (s *BookingService) ReleaseSeats(
	userID string,
	req model.ReleaseSeatsRequest,
) (*model.ReleaseSeatsResponse, error) {

	userID = strings.TrimSpace(userID)
	req.ScheduleID = strings.TrimSpace(req.ScheduleID)
	seats := extractSeats(req.Seat, req.Seats)

	if userID == "" {
		return nil, errors.New("user id is required")
	}

	if req.ScheduleID == "" {
		return nil, errors.New("schedule id is required")
	}

	if len(seats) == 0 {
		return nil, errors.New("at least one seat is required")
	}

	return s.repo.ReleaseSeats(userID, req.ScheduleID, seats)
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

func (s *BookingService) GetBookingDetails(
	userID string,
	bookingRef string,
) (*model.BookingDetailsResponse, error) {

	userID = strings.TrimSpace(userID)
	bookingRef = strings.TrimSpace(bookingRef)

	if userID == "" {
		return nil, errors.New("user id is required")
	}

	if bookingRef == "" {
		return nil, errors.New("booking reference is required")
	}

	return s.repo.GetBookingDetails(
		userID,
		bookingRef,
	)
}
