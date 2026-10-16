package repository

import "ticketdot/internal/operator/model"

type OperatorRepository interface {
	CreateBus(operatorID string, req model.CreateBusRequest) (*model.CreateBusResponse, error)
	CreateRoute(operatorID string, req model.CreateRouteRequest) (*model.CreateRouteResponse, error)
	CreateSchedule(operatorID string, req model.CreateScheduleRequest) (*model.CreateScheduleResponse, error)
	GetBookings(operatorID string) ([]model.OperatorBooking, error)
	GetBuses(operatorID string) ([]model.CreateBusResponse, error)

	GetRoutes(operatorID string) ([]model.RouteResponse, error)
	GetSchedules(operatorID string) ([]model.ScheduleResponse, error)

	GetDashboardStats(operatorID string) (*model.DashboardStats, error)
	GetAnalytics(operatorID string, periodDays int) (*model.AnalyticsOverview, error)

	UpdateBus(busID string, operatorID string, req *model.UpdateBusRequest) error
	DeleteBus(busID string, operatorID string) error
	GetBusLayout(busID string, operatorID string) ([]model.BusSeatLayout, error)
	CancelSchedule(scheduleID string, operatorID string) error
}
