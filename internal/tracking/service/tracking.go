package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"ticketdot/internal/tracking/model"
	"ticketdot/internal/tracking/repository"
)

type TrackingService struct {
	repo repository.TrackingRepository
}

func NewTrackingService(repo repository.TrackingRepository) *TrackingService {
	return &TrackingService{repo: repo}
}

// GetTripTracking retrieves real-time telemetry for a trip identified by schedule ID or booking ref.
func (s *TrackingService) GetTripTracking(ctx context.Context, identifier string) (*model.TripTrackingDetails, error) {
	scheduleID := strings.TrimSpace(identifier)

	// If identifier is not standard UUID length, treat it as a booking ref first
	if len(scheduleID) != 36 {
		if resolvedID, err := s.repo.GetScheduleIDByBookingRef(ctx, scheduleID); err == nil && resolvedID != "" {
			scheduleID = resolvedID
		}
	}

	info, err := s.repo.GetScheduleRouteInfo(ctx, scheduleID)
	if err != nil {
		// Fallback check: maybe identifier is a booking reference after all
		if resolvedID, rErr := s.repo.GetScheduleIDByBookingRef(ctx, identifier); rErr == nil && resolvedID != "" {
			scheduleID = resolvedID
			info, err = s.repo.GetScheduleRouteInfo(ctx, scheduleID)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("trip schedule not found: %w", err)
	}

	// Generate road-interpolated waypoints between Origin and Destination
	waypoints := generateRouteWaypoints(info.OriginLat, info.OriginLng, info.DestLat, info.DestLng, 25)

	// Build physical milestone stops
	stops := s.buildRouteStops(info)

	// Fetch or create tracking state
	tracking, err := s.repo.GetTrackingByScheduleID(ctx, scheduleID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch trip tracking: %w", err)
	}

	now := time.Now()

	// If tracking does not yet exist, initialize default state
	if tracking == nil {
		status := "scheduled"
		speed := 0.0
		progress := 0.0
		heading := calculateBearing(info.OriginLat, info.OriginLng, info.DestLat, info.DestLng)
		currStop := fmt.Sprintf("%s Central Station", info.OriginName)
		nextStop := stops[1].Name
		nextETA := info.DepartureTime.Add(45 * time.Minute)

		if now.After(info.ArrivalTime) {
			status = "completed"
			progress = 100.0
			speed = 0.0
			currStop = fmt.Sprintf("%s Main Terminal", info.DestName)
			nextStop = "Trip Completed"
		} else if now.After(info.DepartureTime) {
			status = "in_transit"
			speed = 64.0
			totalDur := info.ArrivalTime.Sub(info.DepartureTime).Seconds()
			elapsed := now.Sub(info.DepartureTime).Seconds()
			if totalDur > 0 {
				progress = math.Min(95.0, math.Max(5.0, (elapsed/totalDur)*100.0))
			}
		}

		wpIdx := int((progress / 100.0) * float64(len(waypoints)-1))
		currentPt := waypoints[wpIdx]

		tracking = &model.TripTracking{
			ScheduleID:       scheduleID,
			CurrentLatitude:  currentPt.Lat,
			CurrentLongitude: currentPt.Lng,
			SpeedKmh:         speed,
			HeadingDegrees:   heading,
			TripStatus:       status,
			DelayMinutes:     0,
			CurrentStopName:  &currStop,
			NextStopName:     &nextStop,
			NextStopETA:      &nextETA,
			ProgressPercent:  progress,
			LastUpdatedAt:    now,
		}

		_ = s.repo.UpsertTracking(ctx, tracking)
	}

	// Recompute stop completion statuses based on current progress
	for i := range stops {
		stopProgress := float64(i) / float64(len(stops)-1) * 100.0
		if tracking.ProgressPercent >= stopProgress+5.0 {
			stops[i].Status = "passed"
		} else if math.Abs(tracking.ProgressPercent-stopProgress) <= 10.0 {
			stops[i].Status = "current"
		} else {
			stops[i].Status = "upcoming"
		}
	}

	currentStop := ""
	if tracking.CurrentStopName != nil {
		currentStop = *tracking.CurrentStopName
	}
	nextStop := ""
	if tracking.NextStopName != nil {
		nextStop = *tracking.NextStopName
	}

	details := &model.TripTrackingDetails{
		ScheduleID:       info.ScheduleID,
		OperatorName:     info.OperatorName,
		BusType:          info.BusType,
		RegistrationNo:   info.RegistrationNo,
		Origin:           info.OriginName,
		Destination:      info.DestName,
		OriginCoords:     model.Waypoint{Lat: info.OriginLat, Lng: info.OriginLng},
		DestCoords:       model.Waypoint{Lat: info.DestLat, Lng: info.DestLng},
		DepartureTime:    info.DepartureTime,
		ArrivalTime:      info.ArrivalTime,
		TripStatus:       tracking.TripStatus,
		CurrentLatitude:  tracking.CurrentLatitude,
		CurrentLongitude: tracking.CurrentLongitude,
		SpeedKmh:         tracking.SpeedKmh,
		HeadingDegrees:   tracking.HeadingDegrees,
		DelayMinutes:     tracking.DelayMinutes,
		ProgressPercent:  tracking.ProgressPercent,
		CurrentStopName:  currentStop,
		NextStopName:     nextStop,
		NextStopETA:      tracking.NextStopETA,
		LastUpdatedAt:    tracking.LastUpdatedAt,
		Waypoints:        waypoints,
		Stops:            stops,
	}

	return details, nil
}

// UpdateLocation allows the driver telemetry app / operator portal to stream GPS coordinates.
func (s *TrackingService) UpdateLocation(ctx context.Context, scheduleID string, operatorID string, req model.UpdateLocationRequest) error {
	info, err := s.repo.GetScheduleRouteInfo(ctx, scheduleID)
	if err != nil {
		return fmt.Errorf("schedule not found: %w", err)
	}

	if operatorID != "" && info.OperatorID != operatorID {
		return errors.New("unauthorized: operator does not own this schedule")
	}

	tracking := &model.TripTracking{
		ScheduleID:       scheduleID,
		CurrentLatitude:  req.Latitude,
		CurrentLongitude: req.Longitude,
		SpeedKmh:         req.SpeedKmh,
		HeadingDegrees:   req.HeadingDegrees,
		TripStatus:       req.TripStatus,
		DelayMinutes:     req.DelayMinutes,
		CurrentStopName:  &req.CurrentStopName,
		NextStopName:     &req.NextStopName,
		NextStopETA:      req.NextStopETA,
		ProgressPercent:  req.ProgressPercent,
		LastUpdatedAt:    time.Now(),
	}

	return s.repo.UpsertTracking(ctx, tracking)
}

// SimulateTrip advances or adjusts the trip simulation for test benches and live demonstrations.
func (s *TrackingService) SimulateTrip(ctx context.Context, scheduleID string, operatorID string, req model.SimulateTripRequest) (*model.TripTrackingDetails, error) {
	info, err := s.repo.GetScheduleRouteInfo(ctx, scheduleID)
	if err != nil {
		return nil, fmt.Errorf("schedule not found: %w", err)
	}

	if operatorID != "" && info.OperatorID != operatorID {
		return nil, errors.New("unauthorized: operator does not own this schedule")
	}

	waypoints := generateRouteWaypoints(info.OriginLat, info.OriginLng, info.DestLat, info.DestLng, 25)
	stops := s.buildRouteStops(info)

	tracking, _ := s.repo.GetTrackingByScheduleID(ctx, scheduleID)
	if tracking == nil {
		tracking = &model.TripTracking{
			ScheduleID:       scheduleID,
			CurrentLatitude:  info.OriginLat,
			CurrentLongitude: info.OriginLng,
			SpeedKmh:         0,
			HeadingDegrees:   calculateBearing(info.OriginLat, info.OriginLng, info.DestLat, info.DestLng),
			TripStatus:       "scheduled",
			DelayMinutes:     0,
			ProgressPercent:  0,
		}
	}

	now := time.Now()

	switch req.Action {
	case "start":
		tracking.TripStatus = "in_transit"
		tracking.ProgressPercent = 5.0
		tracking.SpeedKmh = 62.0
		tracking.CurrentLatitude = waypoints[1].Lat
		tracking.CurrentLongitude = waypoints[1].Lng
		tracking.HeadingDegrees = calculateBearing(waypoints[0].Lat, waypoints[0].Lng, waypoints[1].Lat, waypoints[1].Lng)
		curr := fmt.Sprintf("%s Boarding Bay 4", info.OriginName)
		next := stops[1].Name
		eta := now.Add(35 * time.Minute)
		tracking.CurrentStopName = &curr
		tracking.NextStopName = &next
		tracking.NextStopETA = &eta

	case "step":
		newProgress := tracking.ProgressPercent + 8.0
		if newProgress >= 100.0 {
			newProgress = 100.0
			tracking.TripStatus = "completed"
			tracking.SpeedKmh = 0
			curr := fmt.Sprintf("%s Main Terminal", info.DestName)
			next := "Trip Completed"
			tracking.CurrentStopName = &curr
			tracking.NextStopName = &next
			tracking.NextStopETA = nil
		} else {
			tracking.TripStatus = "in_transit"
			if tracking.DelayMinutes > 15 {
				tracking.TripStatus = "delayed"
			}
			tracking.SpeedKmh = 68.0 + float64((int(newProgress)%7)-3)
			curr := fmt.Sprintf("Highway Segment (Mile %d)", int(newProgress*4))
			next := stops[len(stops)-1].Name
			if newProgress < 50 && len(stops) > 2 {
				next = stops[1].Name
			}
			eta := now.Add(time.Duration((100-newProgress)*0.8) * time.Minute)
			tracking.CurrentStopName = &curr
			tracking.NextStopName = &next
			tracking.NextStopETA = &eta
		}

		tracking.ProgressPercent = newProgress
		wpIdx := int((newProgress / 100.0) * float64(len(waypoints)-1))
		tracking.CurrentLatitude = waypoints[wpIdx].Lat
		tracking.CurrentLongitude = waypoints[wpIdx].Lng
		if wpIdx < len(waypoints)-1 {
			tracking.HeadingDegrees = calculateBearing(waypoints[wpIdx].Lat, waypoints[wpIdx].Lng, waypoints[wpIdx+1].Lat, waypoints[wpIdx+1].Lng)
		}

	case "delay":
		tracking.DelayMinutes += req.DelayMinutes
		if tracking.DelayMinutes > 10 {
			tracking.TripStatus = "delayed"
		}

	case "complete":
		tracking.ProgressPercent = 100.0
		tracking.TripStatus = "completed"
		tracking.SpeedKmh = 0
		lastWp := waypoints[len(waypoints)-1]
		tracking.CurrentLatitude = lastWp.Lat
		tracking.CurrentLongitude = lastWp.Lng
		curr := fmt.Sprintf("%s Main Terminal", info.DestName)
		next := "Trip Completed"
		tracking.CurrentStopName = &curr
		tracking.NextStopName = &next
		tracking.NextStopETA = nil

	case "reset":
		tracking.ProgressPercent = 0.0
		tracking.TripStatus = "scheduled"
		tracking.SpeedKmh = 0
		tracking.DelayMinutes = 0
		tracking.CurrentLatitude = info.OriginLat
		tracking.CurrentLongitude = info.OriginLng
		curr := fmt.Sprintf("%s Central Station", info.OriginName)
		next := stops[1].Name
		eta := now.Add(45 * time.Minute)
		tracking.CurrentStopName = &curr
		tracking.NextStopName = &next
		tracking.NextStopETA = &eta
	}

	tracking.LastUpdatedAt = now
	if err := s.repo.UpsertTracking(ctx, tracking); err != nil {
		return nil, fmt.Errorf("failed to save simulation state: %w", err)
	}

	return s.GetTripTracking(ctx, scheduleID)
}

func (s *TrackingService) buildRouteStops(info *model.ScheduleRouteInfo) []model.TripStop {
	midLat := (info.OriginLat + info.DestLat) / 2.0
	midLng := (info.OriginLng + info.DestLng) / 2.0

	return []model.TripStop{
		{
			Name:          fmt.Sprintf("%s Central Station", info.OriginName),
			City:          info.OriginName,
			Lat:           info.OriginLat,
			Lng:           info.OriginLng,
			EstimatedTime: info.DepartureTime.Format("15:04 PM"),
			Status:        "passed",
			IsBoarding:    true,
			IsDropping:    false,
		},
		{
			Name:          fmt.Sprintf("%s - %s Highway Toll & Rest Plaza", info.OriginName, info.DestName),
			City:          "Highway NH Expressway",
			Lat:           midLat + 0.015,
			Lng:           midLng - 0.012,
			EstimatedTime: info.DepartureTime.Add(2 * time.Hour).Format("15:04 PM"),
			Status:        "upcoming",
			IsBoarding:    false,
			IsDropping:    false,
		},
		{
			Name:          fmt.Sprintf("%s City Outer Ring Road Junction", info.DestName),
			City:          info.DestName,
			Lat:           info.DestLat - (info.DestLat-info.OriginLat)*0.15,
			Lng:           info.DestLng - (info.DestLng-info.OriginLng)*0.15,
			EstimatedTime: info.ArrivalTime.Add(-30 * time.Minute).Format("15:04 PM"),
			Status:        "upcoming",
			IsBoarding:    false,
			IsDropping:    true,
		},
		{
			Name:          fmt.Sprintf("%s Main Terminal", info.DestName),
			City:          info.DestName,
			Lat:           info.DestLat,
			Lng:           info.DestLng,
			EstimatedTime: info.ArrivalTime.Format("15:04 PM"),
			Status:        "upcoming",
			IsBoarding:    false,
			IsDropping:    true,
		},
	}
}

// generateRouteWaypoints creates a realistic series of path points along highways
func generateRouteWaypoints(lat1, lon1, lat2, lon2 float64, count int) []model.Waypoint {
	if count < 2 {
		count = 2
	}
	points := make([]model.Waypoint, count)
	for i := 0; i < count; i++ {
		t := float64(i) / float64(count-1)
		// Introduce slight natural highway curvature using sinusoidal arc
		curveOffset := math.Sin(t*math.Pi) * 0.045

		lat := lat1 + (lat2-lat1)*t + (curveOffset * 0.7)
		lng := lon1 + (lon2-lon1)*t - (curveOffset * 0.3)

		points[i] = model.Waypoint{
			Lat: math.Round(lat*100000) / 100000,
			Lng: math.Round(lng*100000) / 100000,
		}
	}
	return points
}

func calculateBearing(lat1, lon1, lat2, lon2 float64) float64 {
	radLat1 := lat1 * math.Pi / 180.0
	radLat2 := lat2 * math.Pi / 180.0
	diffLon := (lon2 - lon1) * math.Pi / 180.0

	y := math.Sin(diffLon) * math.Cos(radLat2)
	x := math.Cos(radLat1)*math.Sin(radLat2) -
		math.Sin(radLat1)*math.Cos(radLat2)*math.Cos(diffLon)

	bearing := math.Atan2(y, x) * 180.0 / math.Pi
	return math.Mod(bearing+360.0, 360.0)
}
