package service

import (
	"errors"
	"strings"
	"time"

	"ticketdot/internal/operator/model"
	"ticketdot/internal/operator/repository"
)

func parseFlexibleTime(val string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, val); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("unrecognized time format")
}

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
	operatorID string,
	req model.CreateRouteRequest,
) (*model.CreateRouteResponse, error) {

	operatorID = strings.TrimSpace(operatorID)
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

	return s.repo.CreateRoute(operatorID, req)
}

func (s *OperatorService) CreateSchedule(operatorID string, req model.CreateScheduleRequest) (*model.CreateScheduleResponse, error) {

	operatorID = strings.TrimSpace(operatorID)
	req.RouteID = strings.TrimSpace(req.RouteID)
	req.BusID = strings.TrimSpace(req.BusID)
	req.DepartureTime = strings.TrimSpace(req.DepartureTime)
	req.ArrivalTime = strings.TrimSpace(req.ArrivalTime)

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

	if req.BaseFare <= 0 {
		return nil, errors.New("base fare must be greater than zero")
	}

	depTime, err := parseFlexibleTime(req.DepartureTime)
	if err != nil {
		return nil, errors.New("invalid departure time format")
	}

	arrTime, err := parseFlexibleTime(req.ArrivalTime)
	if err != nil {
		return nil, errors.New("invalid arrival time format")
	}

	if !arrTime.After(depTime) {
		return nil, errors.New("arrival time must be strictly after departure time")
	}

	if !depTime.After(time.Now().Add(-2 * time.Minute)) {
		return nil, errors.New("departure time must be in the future; passengers cannot search or book trips that have already departed")
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

func (s *OperatorService) GetRoutes(
	operatorID string,
) ([]model.RouteResponse, error) {

	if strings.TrimSpace(operatorID) == "" {
		return nil, errors.New("operator id required")
	}

	return s.repo.GetRoutes(operatorID)
}

func (s *OperatorService) GetSchedules(
	operatorID string,
) ([]model.ScheduleResponse, error) {

	operatorID = strings.TrimSpace(operatorID)

	if operatorID == "" {
		return nil, errors.New("operator id required")
	}

	return s.repo.GetSchedules(operatorID)
}

func (s *OperatorService) GetDashboardStats(
	operatorID string,
) (*model.DashboardStats, error) {

	return s.repo.GetDashboardStats(operatorID)
}

func (s *OperatorService) UpdateBus(
	busID string,
	operatorID string,
	req *model.UpdateBusRequest,
) error {
	busID = strings.TrimSpace(busID)
	operatorID = strings.TrimSpace(operatorID)

	if busID == "" {
		return errors.New("bus id is required")
	}
	if operatorID == "" {
		return errors.New("operator id is required")
	}
	if req == nil {
		return errors.New("request payload is required")
	}

	req.RegistrationNo = strings.TrimSpace(strings.ToUpper(req.RegistrationNo))
	req.BusType = strings.TrimSpace(strings.ToUpper(req.BusType))

	if req.RegistrationNo == "" {
		return errors.New("registration number is required")
	}
	if req.BusType == "" {
		return errors.New("bus type is required")
	}
	if req.TotalSeats <= 0 {
		return errors.New("total seats must be greater than zero")
	}
	if req.TotalSeats > 60 {
		return errors.New("total seats cannot exceed 60")
	}

	return s.repo.UpdateBus(
		busID,
		operatorID,
		req,
	)
}

func (s *OperatorService) DeleteBus(
	busID string,
	operatorID string,
) error {
	busID = strings.TrimSpace(busID)
	operatorID = strings.TrimSpace(operatorID)

	if busID == "" {
		return errors.New("bus id is required")
	}
	if operatorID == "" {
		return errors.New("operator id is required")
	}

	return s.repo.DeleteBus(
		busID,
		operatorID,
	)
}

func (s *OperatorService) GetBusLayout(
	busID string,
	operatorID string,
) ([]model.BusSeatLayout, error) {
	busID = strings.TrimSpace(busID)
	operatorID = strings.TrimSpace(operatorID)

	if busID == "" {
		return nil, errors.New("bus id is required")
	}
	if operatorID == "" {
		return nil, errors.New("operator id is required")
	}

	return s.repo.GetBusLayout(
		busID,
		operatorID,
	)
}

func (s *OperatorService) CancelSchedule(
	scheduleID string,
	operatorID string,
) error {
	scheduleID = strings.TrimSpace(scheduleID)
	operatorID = strings.TrimSpace(operatorID)

	if scheduleID == "" {
		return errors.New("schedule id is required")
	}
	if operatorID == "" {
		return errors.New("operator id is required")
	}

	return s.repo.CancelSchedule(
		scheduleID,
		operatorID,
	)
}

func (s *OperatorService) GetAnalytics(
	operatorID string,
	periodDays int,
) (*model.AnalyticsOverview, error) {
	operatorID = strings.TrimSpace(operatorID)
	if operatorID == "" {
		return nil, errors.New("operator id is required")
	}

	if periodDays != 30 {
		periodDays = 7
	}

	return s.repo.GetAnalytics(
		operatorID,
		periodDays,
	)
}
