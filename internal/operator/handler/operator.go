package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	authmiddleware "ticketdot/internal/auth/middleware"
	"ticketdot/internal/httpx"
	"ticketdot/internal/operator/model"
	"ticketdot/internal/operator/service"

	"github.com/go-chi/chi/v5"
)

type OperatorHandler struct {
	service *service.OperatorService
}

func NewOperatorHandler(service *service.OperatorService) *OperatorHandler {
	return &OperatorHandler{
		service: service,
	}
}

func (h *OperatorHandler) CreateBus(w http.ResponseWriter, r *http.Request) {
	operatorID, ok := r.Context().Value(authmiddleware.OperatorIDKey).(string)
	if !ok || operatorID == "" {
		httpx.WriteError(w, r, httpx.Unauthorized("operator id not found"))
		return
	}

	var req model.CreateBusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.Validation("invalid request body"))
		return
	}

	bus, err := h.service.CreateBus(operatorID, req)
	if err != nil {
		httpx.WriteError(w, r, httpx.Validation(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"message": "bus created successfully",
		"data":    bus,
	})
}

func (h *OperatorHandler) CreateRoute(w http.ResponseWriter, r *http.Request) {
	operatorID, ok := r.Context().Value(authmiddleware.OperatorIDKey).(string)
	if !ok || operatorID == "" {
		httpx.WriteError(w, r, httpx.Unauthorized("operator id not found"))
		return
	}

	var req model.CreateRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.Validation("invalid request body"))
		return
	}

	route, err := h.service.CreateRoute(operatorID, req)
	if err != nil {
		httpx.WriteError(w, r, httpx.Validation(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"message": "route created successfully",
		"data":    route,
	})
}

func (h *OperatorHandler) CreateSchedule(w http.ResponseWriter, r *http.Request) {
	operatorID, ok := r.Context().Value(authmiddleware.OperatorIDKey).(string)
	if !ok || operatorID == "" {
		httpx.WriteError(w, r, httpx.Unauthorized("operator id not found"))
		return
	}

	var req model.CreateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.Validation("invalid request body"))
		return
	}

	schedule, err := h.service.CreateSchedule(operatorID, req)
	if err != nil {
		httpx.WriteError(w, r, httpx.Validation(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"message": "schedule created successfully",
		"data":    schedule,
	})
}

func (h *OperatorHandler) GetBookings(w http.ResponseWriter, r *http.Request) {
	operatorID, ok := r.Context().Value(authmiddleware.OperatorIDKey).(string)
	if !ok || operatorID == "" {
		httpx.WriteError(w, r, httpx.Unauthorized("operator id not found"))
		return
	}

	bookings, err := h.service.GetBookings(operatorID)
	if err != nil {
		httpx.WriteError(w, r, httpx.Validation(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"count": len(bookings),
		"data":  bookings,
	})
}

func (h *OperatorHandler) GetBuses(w http.ResponseWriter, r *http.Request) {
	operatorID, ok := r.Context().Value(authmiddleware.OperatorIDKey).(string)
	if !ok || operatorID == "" {
		httpx.WriteError(w, r, httpx.Unauthorized("operator id not found"))
		return
	}

	buses, err := h.service.GetBuses(operatorID)
	if err != nil {
		httpx.WriteError(w, r, httpx.Validation(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"count": len(buses),
		"data":  buses,
	})
}

func (h *OperatorHandler) GetRoutes(w http.ResponseWriter, r *http.Request) {
	operatorID, ok := r.Context().Value(authmiddleware.OperatorIDKey).(string)
	if !ok || operatorID == "" {
		httpx.WriteError(w, r, httpx.Unauthorized("operator id not found"))
		return
	}

	routes, err := h.service.GetRoutes(operatorID)
	if err != nil {
		httpx.WriteError(w, r, httpx.Internal(err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"count": len(routes),
		"data":  routes,
	})
}

func (h *OperatorHandler) GetSchedules(w http.ResponseWriter, r *http.Request) {
	operatorID, ok := r.Context().Value(authmiddleware.OperatorIDKey).(string)
	if !ok || operatorID == "" {
		httpx.WriteError(w, r, httpx.Unauthorized("operator id not found"))
		return
	}

	schedules, err := h.service.GetSchedules(operatorID)
	if err != nil {
		httpx.WriteError(w, r, httpx.Internal(err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"count": len(schedules),
		"data":  schedules,
	})
}

func (h *OperatorHandler) GetDashboardStats(w http.ResponseWriter, r *http.Request) {
	operatorID, ok := r.Context().Value(authmiddleware.OperatorIDKey).(string)
	if !ok || operatorID == "" {
		httpx.WriteError(w, r, httpx.Unauthorized("operator not found"))
		return
	}

	stats, err := h.service.GetDashboardStats(operatorID)
	if err != nil {
		httpx.WriteError(w, r, httpx.Internal(err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": stats,
	})
}

func (h *OperatorHandler) UpdateBus(w http.ResponseWriter, r *http.Request) {
	busID := chi.URLParam(r, "busID")
	operatorID, ok := r.Context().Value(authmiddleware.OperatorIDKey).(string)
	if !ok || operatorID == "" {
		httpx.WriteError(w, r, httpx.Unauthorized("operator id not found"))
		return
	}

	var req model.UpdateBusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.Validation("invalid request body"))
		return
	}

	err := h.service.UpdateBus(busID, operatorID, &req)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteError(w, r, httpx.NotFound("bus not found"))
			return
		}

		errMsg := err.Error()
		if strings.Contains(errMsg, "cannot change seating capacity") ||
			strings.Contains(errMsg, "already exists") ||
			strings.Contains(errMsg, "required") ||
			strings.Contains(errMsg, "greater than zero") ||
			strings.Contains(errMsg, "cannot exceed") {
			httpx.WriteError(w, r, httpx.Validation(errMsg))
			return
		}

		httpx.WriteError(w, r, httpx.Internal(err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Bus updated successfully",
	})
}

func (h *OperatorHandler) DeleteBus(w http.ResponseWriter, r *http.Request) {
	busID := chi.URLParam(r, "busID")
	operatorID, ok := r.Context().Value(authmiddleware.OperatorIDKey).(string)
	if !ok || operatorID == "" {
		httpx.WriteError(w, r, httpx.Unauthorized("operator id not found"))
		return
	}

	err := h.service.DeleteBus(busID, operatorID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteError(w, r, httpx.NotFound("bus not found"))
			return
		}

		errMsg := err.Error()
		if strings.Contains(errMsg, "cannot delete bus") {
			httpx.WriteError(w, r, httpx.Validation(errMsg))
			return
		}

		httpx.WriteError(w, r, httpx.Internal(err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Bus deleted successfully",
	})
}

func (h *OperatorHandler) GetBusLayout(w http.ResponseWriter, r *http.Request) {
	busID := chi.URLParam(r, "busID")
	operatorID, ok := r.Context().Value(authmiddleware.OperatorIDKey).(string)
	if !ok || operatorID == "" {
		httpx.WriteError(w, r, httpx.Unauthorized("operator id not found"))
		return
	}

	layout, err := h.service.GetBusLayout(busID, operatorID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteError(w, r, httpx.NotFound("bus not found"))
			return
		}
		httpx.WriteError(w, r, httpx.Internal(err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"bus_id": busID,
		"count":  len(layout),
		"data":   layout,
	})
}

func (h *OperatorHandler) CancelSchedule(w http.ResponseWriter, r *http.Request) {
	scheduleID := chi.URLParam(r, "scheduleID")
	operatorID, ok := r.Context().Value(authmiddleware.OperatorIDKey).(string)
	if !ok || operatorID == "" {
		httpx.WriteError(w, r, httpx.Unauthorized("operator id not found"))
		return
	}

	err := h.service.CancelSchedule(scheduleID, operatorID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteError(w, r, httpx.NotFound("schedule not found"))
			return
		}

		errMsg := err.Error()
		if strings.Contains(errMsg, "already cancelled") ||
			strings.Contains(errMsg, "cannot cancel a completed trip") ||
			strings.Contains(errMsg, "required") {
			httpx.WriteError(w, r, httpx.Validation(errMsg))
			return
		}

		httpx.WriteError(w, r, httpx.Internal(err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Schedule cancelled successfully",
	})
}

func (h *OperatorHandler) GetAnalytics(w http.ResponseWriter, r *http.Request) {
	operatorID, ok := r.Context().Value(authmiddleware.OperatorIDKey).(string)
	if !ok || operatorID == "" {
		httpx.WriteError(w, r, httpx.Unauthorized("operator id not found"))
		return
	}

	periodParam := r.URL.Query().Get("period")
	periodDays := 7
	if periodParam == "30d" || periodParam == "30" {
		periodDays = 30
	}

	analytics, err := h.service.GetAnalytics(operatorID, periodDays)
	if err != nil {
		httpx.WriteError(w, r, httpx.Internal(err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"data": analytics,
	})
}
