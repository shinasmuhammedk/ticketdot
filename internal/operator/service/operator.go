package service

import (
	"errors"
	"strings"

	"ticketdot/internal/operator/model"
	"ticketdot/internal/operator/repository"
)

type OperatorService struct {
	repo repository.OperatorRepository
}

func NewOperatorService(repo repository.OperatorRepository) *OperatorService {
	return &OperatorService{
		repo: repo,
	}
}

func (s *OperatorService) CreateBus(
	operatorID string,
	req model.CreateBusRequest,
) (*model.CreateBusResponse, error) {

	operatorID = strings.TrimSpace(operatorID)
	req.RegistrationNo = strings.TrimSpace(strings.ToUpper(req.RegistrationNo))
	req.BusType = strings.TrimSpace(strings.ToUpper(req.BusType))

	if operatorID == "" {
		return nil, errors.New("operator id is required")
	}

	if req.RegistrationNo == "" {
		return nil, errors.New("registration number is required")
	}

	if req.BusType == "" {
		return nil, errors.New("bus type is required")
	}

	if req.TotalSeats <= 0 {
		return nil, errors.New("total seats must be greater than zero")
	}

	return s.repo.CreateBus(operatorID, req)
}

func (s *OperatorService) CreateRoute(
	req model.CreateRouteRequest,
) (*model.CreateRouteResponse, error) {

	req.OriginCityID = strings.TrimSpace(req.OriginCityID)
	req.DestinationCityID = strings.TrimSpace(req.DestinationCityID)

	if req.OriginCityID == "" {
		return nil, errors.New("origin city id is required")
	}

	if req.DestinationCityID == "" {
		return nil, errors.New("destination city id is required")
	}

	if req.OriginCityID == req.DestinationCityID {
		return nil, errors.New("origin and destination cannot be the same")
	}

	return s.repo.CreateRoute(req)
}

func (s *OperatorService) CreateSchedule(operatorID string, req model.CreateScheduleRequest) (*model.CreateScheduleResponse, error) {

	operatorID = strings.TrimSpace(operatorID)
	req.RouteID = strings.TrimSpace(req.RouteID)
	req.BusID = strings.TrimSpace(req.BusID)

	if operatorID == "" {
		return nil, errors.New("operator id is required")
	}

	if req.RouteID == "" {
		return nil, errors.New("route id is required")
	}

	if req.BusID == "" {
		return nil, errors.New("bus id is required")
	}

	if req.DepartureTime == "" {
		return nil, errors.New("departure time is required")
	}

	if req.ArrivalTime == "" {
		return nil, errors.New("arrival time is required")
	}

	if req.AvailableSeats <= 0 {
		return nil, errors.New("available seats must be greater than zero")
	}

	if req.BaseFare <= 0 {
		return nil, errors.New("base fare must be greater than zero")
	}

	return s.repo.CreateSchedule(operatorID, req)
}

func (s *OperatorService) GetBookings(
	operatorID string,
) ([]model.OperatorBooking, error) {

	operatorID = strings.TrimSpace(operatorID)

	if operatorID == "" {
		return nil, errors.New("operator id is required")
	}

	return s.repo.GetBookings(operatorID)
}

func (s *OperatorService) GetBuses(
	operatorID string,
) ([]model.CreateBusResponse, error) {

	operatorID = strings.TrimSpace(operatorID)

	if operatorID == "" {
		return nil, errors.New("operator id is required")
	}

	return s.repo.GetBuses(operatorID)
}
