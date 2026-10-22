package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	authmiddleware "ticketdot/internal/auth/middleware"
	"ticketdot/internal/httpx"
	"ticketdot/internal/tracking/model"
	"ticketdot/internal/tracking/service"

	"github.com/go-chi/chi/v5"
)

type TrackingHandler struct {
	service *service.TrackingService
}

func NewTrackingHandler(service *service.TrackingService) *TrackingHandler {
	return &TrackingHandler{service: service}
}

// GetTracking serves real-time GPS telemetry, route polyline, and stop progression
func (h *TrackingHandler) GetTracking(w http.ResponseWriter, r *http.Request) {
	identifier := chi.URLParam(r, "identifier")
	if identifier == "" {
		httpx.WriteError(w, r, httpx.Validation("missing trip or booking identifier"))
		return
	}

	details, err := h.service.GetTripTracking(r.Context(), identifier)
	if err != nil {
		httpx.WriteError(w, r, httpx.NotFound(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"message": "tracking telemetry retrieved",
		"data":    details,
	})
}

// UpdateLocation receives live driver GPS pings
func (h *TrackingHandler) UpdateLocation(w http.ResponseWriter, r *http.Request) {
	scheduleID := chi.URLParam(r, "scheduleID")
	if scheduleID == "" {
		httpx.WriteError(w, r, httpx.Validation("missing schedule ID"))
		return
	}

	operatorID, _ := r.Context().Value(authmiddleware.OperatorIDKey).(string)

	var req model.UpdateLocationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.Validation("invalid request body"))
		return
	}

	if err := h.service.UpdateLocation(r.Context(), scheduleID, operatorID, req); err != nil {
		if strings.Contains(err.Error(), "unauthorized") {
			httpx.WriteError(w, r, httpx.Forbidden(err.Error()))
			return
		}
		httpx.WriteError(w, r, httpx.Validation(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"message": "location telemetry updated successfully",
	})
}

// SimulateTrip advances or adjusts trip progress for interactive demo and test benches
func (h *TrackingHandler) SimulateTrip(w http.ResponseWriter, r *http.Request) {
	scheduleID := chi.URLParam(r, "scheduleID")
	if scheduleID == "" {
		httpx.WriteError(w, r, httpx.Validation("missing schedule ID"))
		return
	}

	operatorID, _ := r.Context().Value(authmiddleware.OperatorIDKey).(string)

	var req model.SimulateTripRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Action = "step"
	}

	updated, err := h.service.SimulateTrip(r.Context(), scheduleID, operatorID, req)
	if err != nil {
		if strings.Contains(err.Error(), "unauthorized") {
			httpx.WriteError(w, r, httpx.Forbidden(err.Error()))
			return
		}
		httpx.WriteError(w, r, httpx.Validation(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"message": "simulation action executed successfully",
		"data":    updated,
	})
}
