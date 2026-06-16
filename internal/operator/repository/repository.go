package repository

import "ticketdot/internal/operator/model"

type OperatorRepository interface {
	CreateBus(operatorID string, req model.CreateBusRequest) (*model.CreateBusResponse, error)
	CreateRoute(req model.CreateRouteRequest) (*model.CreateRouteResponse, error)
	CreateSchedule(operatorID string, req model.CreateScheduleRequest) (*model.CreateScheduleResponse, error)
    GetBookings(operatorID string) ([]model.OperatorBooking, error)
}
