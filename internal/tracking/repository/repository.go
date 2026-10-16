package repository

import (
	"context"
	"ticketdot/internal/tracking/model"
)

type TrackingRepository interface {
	GetTrackingByScheduleID(ctx context.Context, scheduleID string) (*model.TripTracking, error)
	UpsertTracking(ctx context.Context, tracking *model.TripTracking) error
	GetScheduleRouteInfo(ctx context.Context, scheduleID string) (*model.ScheduleRouteInfo, error)
	GetScheduleIDByBookingRef(ctx context.Context, bookingRef string) (string, error)
}
