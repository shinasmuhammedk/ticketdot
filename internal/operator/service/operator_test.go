package service

import (
	"errors"
	"testing"
	"time"

	"ticketdot/internal/operator/model"
)

type mockOperatorRepo struct {
	createBusFn        func(operatorID string, req model.CreateBusRequest) (*model.CreateBusResponse, error)
	createRouteFn      func(operatorID string, req model.CreateRouteRequest) (*model.CreateRouteResponse, error)
	createScheduleFn   func(operatorID string, req model.CreateScheduleRequest) (*model.CreateScheduleResponse, error)
	getBookingsFn      func(operatorID string) ([]model.OperatorBooking, error)
	getBusesFn         func(operatorID string) ([]model.CreateBusResponse, error)
	getRoutesFn        func(operatorID string) ([]model.RouteResponse, error)
	getSchedulesFn     func(operatorID string) ([]model.ScheduleResponse, error)
	getDashboardStatsFn func(operatorID string) (*model.DashboardStats, error)
	updateBusFn        func(busID string, operatorID string, req *model.UpdateBusRequest) error
	deleteBusFn        func(busID string, operatorID string) error
	getBusLayoutFn     func(busID string, operatorID string) ([]model.BusSeatLayout, error)
	cancelScheduleFn   func(scheduleID string, operatorID string) error
	getAnalyticsFn     func(operatorID string, periodDays int) (*model.AnalyticsOverview, error)
}

func (m *mockOperatorRepo) CreateBus(operatorID string, req model.CreateBusRequest) (*model.CreateBusResponse, error) {
	if m.createBusFn != nil {
		return m.createBusFn(operatorID, req)
	}
	return &model.CreateBusResponse{ID: "bus-1", RegistrationNo: req.RegistrationNo, BusType: req.BusType, TotalSeats: req.TotalSeats}, nil
}

func (m *mockOperatorRepo) CreateRoute(operatorID string, req model.CreateRouteRequest) (*model.CreateRouteResponse, error) {
	if m.createRouteFn != nil {
		return m.createRouteFn(operatorID, req)
	}
	return &model.CreateRouteResponse{ID: "route-1", OriginCityID: req.OriginCityID, DestinationCityID: req.DestinationCityID}, nil
}

func (m *mockOperatorRepo) CreateSchedule(operatorID string, req model.CreateScheduleRequest) (*model.CreateScheduleResponse, error) {
	if m.createScheduleFn != nil {
		return m.createScheduleFn(operatorID, req)
	}
	return &model.CreateScheduleResponse{ID: "sched-1", RouteID: req.RouteID, BusID: req.BusID, BaseFare: req.BaseFare}, nil
}

func (m *mockOperatorRepo) GetBookings(operatorID string) ([]model.OperatorBooking, error) {
	if m.getBookingsFn != nil {
		return m.getBookingsFn(operatorID)
	}
	return nil, nil
}

func (m *mockOperatorRepo) GetBuses(operatorID string) ([]model.CreateBusResponse, error) {
	if m.getBusesFn != nil {
		return m.getBusesFn(operatorID)
	}
	return nil, nil
}

func (m *mockOperatorRepo) GetRoutes(operatorID string) ([]model.RouteResponse, error) {
	if m.getRoutesFn != nil {
		return m.getRoutesFn(operatorID)
	}
	return nil, nil
}

func (m *mockOperatorRepo) GetSchedules(operatorID string) ([]model.ScheduleResponse, error) {
	if m.getSchedulesFn != nil {
		return m.getSchedulesFn(operatorID)
	}
	return nil, nil
}

func (m *mockOperatorRepo) GetDashboardStats(operatorID string) (*model.DashboardStats, error) {
	if m.getDashboardStatsFn != nil {
		return m.getDashboardStatsFn(operatorID)
	}
	return &model.DashboardStats{}, nil
}

func (m *mockOperatorRepo) UpdateBus(busID string, operatorID string, req *model.UpdateBusRequest) error {
	if m.updateBusFn != nil {
		return m.updateBusFn(busID, operatorID, req)
	}
	return nil
}

func (m *mockOperatorRepo) DeleteBus(busID string, operatorID string) error {
	if m.deleteBusFn != nil {
		return m.deleteBusFn(busID, operatorID)
	}
	return nil
}

func (m *mockOperatorRepo) GetBusLayout(busID string, operatorID string) ([]model.BusSeatLayout, error) {
	if m.getBusLayoutFn != nil {
		return m.getBusLayoutFn(busID, operatorID)
	}
	return nil, nil
}

func (m *mockOperatorRepo) CancelSchedule(scheduleID string, operatorID string) error {
	if m.cancelScheduleFn != nil {
		return m.cancelScheduleFn(scheduleID, operatorID)
	}
	return nil
}

func (m *mockOperatorRepo) GetAnalytics(operatorID string, periodDays int) (*model.AnalyticsOverview, error) {
	if m.getAnalyticsFn != nil {
		return m.getAnalyticsFn(operatorID, periodDays)
	}
	return &model.AnalyticsOverview{}, nil
}

func TestCreateSchedule_Validation(t *testing.T) {
	mock := &mockOperatorRepo{}
	svc := NewOperatorService(mock)

	now := time.Now().UTC()
	dep := now.Add(2 * time.Hour).Format(time.RFC3339)
	arr := now.Add(6 * time.Hour).Format(time.RFC3339)

	// Valid schedule
	_, err := svc.CreateSchedule("op-1", model.CreateScheduleRequest{
		RouteID:       "route-1",
		BusID:         "bus-1",
		DepartureTime: dep,
		ArrivalTime:   arr,
		BaseFare:      500,
	})
	if err != nil {
		t.Fatalf("expected valid schedule, got error: %v", err)
	}

	// Arrival before departure
	_, err = svc.CreateSchedule("op-1", model.CreateScheduleRequest{
		RouteID:       "route-1",
		BusID:         "bus-1",
		DepartureTime: arr,
		ArrivalTime:   dep,
		BaseFare:      500,
	})
	if err == nil {
		t.Fatal("expected error when arrival is before departure, got nil")
	}

	// Missing route
	_, err = svc.CreateSchedule("op-1", model.CreateScheduleRequest{
		RouteID:       "",
		BusID:         "bus-1",
		DepartureTime: dep,
		ArrivalTime:   arr,
		BaseFare:      500,
	})
	if err == nil {
		t.Fatal("expected error on empty route id, got nil")
	}

	// Non-positive fare
	_, err = svc.CreateSchedule("op-1", model.CreateScheduleRequest{
		RouteID:       "route-1",
		BusID:         "bus-1",
		DepartureTime: dep,
		ArrivalTime:   arr,
		BaseFare:      0,
	})
	if err == nil {
		t.Fatal("expected error on zero base fare, got nil")
	}
}

func TestUpdateBus_Validation(t *testing.T) {
	mock := &mockOperatorRepo{}
	svc := NewOperatorService(mock)

	// Valid update
	err := svc.UpdateBus("bus-1", "op-1", &model.UpdateBusRequest{
		RegistrationNo: "mh-12-ab-1234",
		BusType:        "ac_sleeper",
		TotalSeats:     36,
	})
	if err != nil {
		t.Fatalf("expected valid update, got %v", err)
	}

	// Empty registration
	err = svc.UpdateBus("bus-1", "op-1", &model.UpdateBusRequest{
		RegistrationNo: "",
		BusType:        "ac_sleeper",
		TotalSeats:     36,
	})
	if err == nil {
		t.Fatal("expected error on empty reg, got nil")
	}

	// Invalid seat count
	err = svc.UpdateBus("bus-1", "op-1", &model.UpdateBusRequest{
		RegistrationNo: "MH-12-1234",
		BusType:        "AC_SLEEPER",
		TotalSeats:     0,
	})
	if err == nil {
		t.Fatal("expected error on 0 seats, got nil")
	}

	// Excessive seat count (> 60)
	err = svc.UpdateBus("bus-1", "op-1", &model.UpdateBusRequest{
		RegistrationNo: "MH-12-1234",
		BusType:        "AC_SLEEPER",
		TotalSeats:     99,
	})
	if err == nil {
		t.Fatal("expected error on >60 seats, got nil")
	}
}

func TestDeleteBus_Validation(t *testing.T) {
	mock := &mockOperatorRepo{
		deleteBusFn: func(busID string, operatorID string) error {
			if busID == "bus-with-schedules" {
				return errors.New("cannot delete bus: vehicle is assigned to active trip schedules")
			}
			return nil
		},
	}
	svc := NewOperatorService(mock)

	// Safe delete
	err := svc.DeleteBus("bus-empty", "op-1")
	if err != nil {
		t.Fatalf("expected successful delete, got %v", err)
	}

	// Bus with active schedules
	err = svc.DeleteBus("bus-with-schedules", "op-1")
	if err == nil {
		t.Fatal("expected error when bus has schedules, got nil")
	}
}

func TestCancelSchedule_Validation(t *testing.T) {
	mock := &mockOperatorRepo{
		cancelScheduleFn: func(scheduleID string, operatorID string) error {
			if scheduleID == "already-cancelled" {
				return errors.New("schedule is already cancelled")
			}
			if scheduleID == "completed-trip" {
				return errors.New("cannot cancel a completed trip schedule")
			}
			return nil
		},
	}
	svc := NewOperatorService(mock)

	// Valid cancel
	err := svc.CancelSchedule("sched-active", "op-1")
	if err != nil {
		t.Fatalf("expected successful cancel, got %v", err)
	}

	// Empty schedule id
	err = svc.CancelSchedule("", "op-1")
	if err == nil {
		t.Fatal("expected error on empty schedule id, got nil")
	}

	// Already cancelled
	err = svc.CancelSchedule("already-cancelled", "op-1")
	if err == nil {
		t.Fatal("expected error on already cancelled schedule, got nil")
	}

	// Completed trip
	err = svc.CancelSchedule("completed-trip", "op-1")
	if err == nil {
		t.Fatal("expected error on completed trip, got nil")
	}
}

func TestGetAnalytics_Validation(t *testing.T) {
	mock := &mockOperatorRepo{
		getAnalyticsFn: func(operatorID string, periodDays int) (*model.AnalyticsOverview, error) {
			return &model.AnalyticsOverview{
				Period:       "7d",
				TotalRevenue: 15000,
				TotalTickets: 25,
			}, nil
		},
	}
	svc := NewOperatorService(mock)

	// Valid analytics call
	res, err := svc.GetAnalytics("op-1", 7)
	if err != nil {
		t.Fatalf("expected successful analytics call, got %v", err)
	}
	if res.TotalRevenue != 15000 {
		t.Errorf("expected 15000 revenue, got %f", res.TotalRevenue)
	}

	// Empty operator id
	_, err = svc.GetAnalytics("", 7)
	if err == nil {
		t.Fatal("expected error on empty operator id, got nil")
	}
}
