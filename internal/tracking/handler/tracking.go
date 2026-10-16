package handler

import (
	"encoding/json"
	"net/http"

	authmiddleware "ticketdot/internal/auth/middleware"
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
		http.Error(w, "missing trip or booking identifier", http.StatusBadRequest)
		return
	}

	details, err := h.service.GetTripTracking(r.Context(), identifier)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
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
		http.Error(w, "missing schedule ID", http.StatusBadRequest)
		return
	}

	operatorID, _ := r.Context().Value(authmiddleware.UserIDKey).(string)

	var req model.UpdateLocationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.service.UpdateLocation(r.Context(), scheduleID, operatorID, req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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
		http.Error(w, "missing schedule ID", http.StatusBadRequest)
		return
	}

	operatorID, _ := r.Context().Value(authmiddleware.UserIDKey).(string)

	var req model.SimulateTripRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// default to step action if empty
		req.Action = "step"
	}

	updated, err := h.service.SimulateTrip(r.Context(), scheduleID, operatorID, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"message": "simulation action executed successfully",
		"data":    updated,
	})
}
