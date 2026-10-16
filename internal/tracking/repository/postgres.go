package repository

import (
	"context"
	"database/sql"
	"errors"
	"ticketdot/internal/tracking/model"

	"github.com/jmoiron/sqlx"
)

type PostgresTrackingRepository struct {
	db *sqlx.DB
}

func NewPostgresTrackingRepository(db *sqlx.DB) *PostgresTrackingRepository {
	return &PostgresTrackingRepository{db: db}
}

func (r *PostgresTrackingRepository) GetTrackingByScheduleID(ctx context.Context, scheduleID string) (*model.TripTracking, error) {
	query := `
		SELECT 
			id, schedule_id, current_latitude, current_longitude, speed_kmh,
			heading_degrees, trip_status, delay_minutes, current_stop_name,
			next_stop_name, next_stop_eta, progress_percent, last_updated_at
		FROM trip_trackings
		WHERE schedule_id = $1
	`
	var tracking model.TripTracking
	if err := r.db.GetContext(ctx, &tracking, query, scheduleID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &tracking, nil
}

func (r *PostgresTrackingRepository) UpsertTracking(ctx context.Context, tracking *model.TripTracking) error {
	query := `
		INSERT INTO trip_trackings (
			schedule_id, current_latitude, current_longitude, speed_kmh, heading_degrees,
			trip_status, delay_minutes, current_stop_name, next_stop_name, next_stop_eta,
			progress_percent, last_updated_at
		) VALUES (
			:schedule_id, :current_latitude, :current_longitude, :speed_kmh, :heading_degrees,
			:trip_status, :delay_minutes, :current_stop_name, :next_stop_name, :next_stop_eta,
			:progress_percent, NOW()
		)
		ON CONFLICT (schedule_id) DO UPDATE SET
			current_latitude = EXCLUDED.current_latitude,
			current_longitude = EXCLUDED.current_longitude,
			speed_kmh = EXCLUDED.speed_kmh,
			heading_degrees = EXCLUDED.heading_degrees,
			trip_status = EXCLUDED.trip_status,
			delay_minutes = EXCLUDED.delay_minutes,
			current_stop_name = EXCLUDED.current_stop_name,
			next_stop_name = EXCLUDED.next_stop_name,
			next_stop_eta = EXCLUDED.next_stop_eta,
			progress_percent = EXCLUDED.progress_percent,
			last_updated_at = NOW()
	`
	_, err := r.db.NamedExecContext(ctx, query, tracking)
	return err
}

func (r *PostgresTrackingRepository) GetScheduleRouteInfo(ctx context.Context, scheduleID string) (*model.ScheduleRouteInfo, error) {
	query := `
		SELECT 
			s.id AS schedule_id,
			s.operator_id,
			o.name AS operator_name,
			b.id AS bus_id,
			b.bus_type,
			b.registration_no,
			s.departure_time,
			s.arrival_time,
			co.name AS origin_name,
			COALESCE(co.latitude, 12.9716) AS origin_lat,
			COALESCE(co.longitude, 77.5946) AS origin_lng,
			cd.name AS dest_name,
			COALESCE(cd.latitude, 13.0827) AS dest_lat,
			COALESCE(cd.longitude, 80.2707) AS dest_lng,
			s.status AS schedule_status
		FROM schedules s
		JOIN operators o ON s.operator_id = o.id
		JOIN buses b ON s.bus_id = b.id
		JOIN routes r ON s.route_id = r.id
		JOIN cities co ON r.origin_id = co.id
		JOIN cities cd ON r.destination_id = cd.id
		WHERE s.id = $1
	`
	var info model.ScheduleRouteInfo
	if err := r.db.GetContext(ctx, &info, query, scheduleID); err != nil {
		return nil, err
	}
	return &info, nil
}

func (r *PostgresTrackingRepository) GetScheduleIDByBookingRef(ctx context.Context, bookingRef string) (string, error) {
	query := `SELECT schedule_id FROM bookings WHERE booking_ref = $1 LIMIT 1`
	var scheduleID string
	if err := r.db.GetContext(ctx, &scheduleID, query, bookingRef); err != nil {
		return "", err
	}
	return scheduleID, nil
}
