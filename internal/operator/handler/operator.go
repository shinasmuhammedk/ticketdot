package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	authmiddleware "ticketdot/internal/auth/middleware"
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
	if !ok {
		http.Error(w, "operator id not found", http.StatusUnauthorized)
		return
	}
	// operatorID := "acc0ab14-6ef9-4266-bdc0-914ebdd93807"

	log.Println("Operator ID:", operatorID)

	var req model.CreateBusRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	bus, err := h.service.CreateBus(operatorID, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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
	operatorID, _ := r.Context().Value(authmiddleware.OperatorIDKey).(string)

	var req model.CreateRouteRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	route, err := h.service.CreateRoute(operatorID, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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
	// operatorID := "acc0ab14-6ef9-4266-bdc0-914ebdd93807"
	operatorID, ok := r.Context().Value(authmiddleware.OperatorIDKey).(string)
	if !ok {
		http.Error(w, "operator id not found", http.StatusUnauthorized)
		return
	}

	var req model.CreateScheduleRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	schedule, err := h.service.CreateSchedule(operatorID, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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
	// operatorID := "acc0ab14-6ef9-4266-bdc0-914ebdd93807"

	operatorID, ok := r.Context().Value(authmiddleware.OperatorIDKey).(string)
	if !ok {
		http.Error(w, "operator id not found", http.StatusUnauthorized)
		return
	}

	bookings, err := h.service.GetBookings(operatorID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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
	if !ok {
		http.Error(w, "operator id not found", http.StatusUnauthorized)
		return
	}

	buses, err := h.service.GetBuses(operatorID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(map[string]any{
		"count": len(buses),
		"data":  buses,
	})
}

func (h *OperatorHandler) GetRoutes(
	w http.ResponseWriter,
	r *http.Request,
) {

	operatorID, ok := r.Context().
		Value(authmiddleware.OperatorIDKey).(string)

	if !ok {
		http.Error(
			w,
			"unauthorized",
			http.StatusUnauthorized,
		)
		return
	}

	routes, err := h.service.GetRoutes(
		operatorID,
	)

	if err != nil {
		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	json.NewEncoder(w).Encode(map[string]any{
		"count": len(routes),
		"data":  routes,
	})
}

func (h *OperatorHandler) GetSchedules(
	w http.ResponseWriter,
	r *http.Request,
) {

	operatorID, ok := r.Context().
		Value(authmiddleware.OperatorIDKey).(string)

	if !ok {
		http.Error(
			w,
			"unauthorized",
			http.StatusUnauthorized,
		)
		return
	}

	schedules, err := h.service.GetSchedules(
		operatorID,
	)

	if err != nil {
		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	json.NewEncoder(w).Encode(map[string]any{
		"count": len(schedules),
		"data":  schedules,
	})
}

func (h *OperatorHandler) GetDashboardStats(
	w http.ResponseWriter,
	r *http.Request,
) {

	operatorID, ok := r.Context().
		Value(authmiddleware.OperatorIDKey).(string)

	if !ok {
		http.Error(
			w,
			"operator not found",
			http.StatusUnauthorized,
		)
		return
	}

	stats, err := h.service.GetDashboardStats(
		operatorID,
	)

	if err != nil {
		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	json.NewEncoder(w).Encode(
		map[string]any{
			"data": stats,
		},
	)
}

func (h *OperatorHandler) UpdateBus(
	w http.ResponseWriter,
	r *http.Request,
) {
	w.Header().Set("Content-Type", "application/json")
	busID := chi.URLParam(r, "busID")

	operatorID, ok := r.Context().
		Value(authmiddleware.OperatorIDKey).(string)

	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "operator id not found",
		})
		return
	}

	var req model.UpdateBusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "invalid request body",
		})
		return
	}

	err := h.service.UpdateBus(
		busID,
		operatorID,
		&req,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"message": "bus not found",
			})
			return
		}

		errMsg := err.Error()
		if strings.Contains(errMsg, "cannot change seating capacity") ||
			strings.Contains(errMsg, "already exists") ||
			strings.Contains(errMsg, "required") ||
			strings.Contains(errMsg, "greater than zero") ||
			strings.Contains(errMsg, "cannot exceed") {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"message": errMsg,
			})
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"message": errMsg,
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Bus updated successfully",
	})
}

func (h *OperatorHandler) DeleteBus(
	w http.ResponseWriter,
	r *http.Request,
) {
	w.Header().Set("Content-Type", "application/json")
	busID := chi.URLParam(r, "busID")

	operatorID, ok := r.Context().
		Value(authmiddleware.OperatorIDKey).(string)

	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "operator id not found",
		})
		return
	}

	err := h.service.DeleteBus(
		busID,
		operatorID,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"message": "bus not found",
			})
			return
		}

		errMsg := err.Error()
		if strings.Contains(errMsg, "cannot delete bus") {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"message": errMsg,
			})
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"message": errMsg,
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Bus deleted successfully",
	})
}

func (h *OperatorHandler) GetBusLayout(
	w http.ResponseWriter,
	r *http.Request,
) {
	w.Header().Set("Content-Type", "application/json")
	busID := chi.URLParam(r, "busID")

	operatorID, ok := r.Context().
		Value(authmiddleware.OperatorIDKey).(string)

	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "operator id not found",
		})
		return
	}

	layout, err := h.service.GetBusLayout(busID, operatorID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"message": "bus not found",
			})
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"message": err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"bus_id": busID,
		"count":  len(layout),
		"data":   layout,
	})
}

func (h *OperatorHandler) CancelSchedule(
	w http.ResponseWriter,
	r *http.Request,
) {
	w.Header().Set("Content-Type", "application/json")
	scheduleID := chi.URLParam(r, "scheduleID")

	operatorID, ok := r.Context().
		Value(authmiddleware.OperatorIDKey).(string)

	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "operator id not found",
		})
		return
	}

	err := h.service.CancelSchedule(scheduleID, operatorID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"message": "schedule not found",
			})
			return
		}

		errMsg := err.Error()
		if strings.Contains(errMsg, "already cancelled") ||
			strings.Contains(errMsg, "cannot cancel a completed trip") ||
			strings.Contains(errMsg, "required") {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"message": errMsg,
			})
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"message": errMsg,
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Schedule cancelled successfully",
	})
}

func (h *OperatorHandler) GetAnalytics(
	w http.ResponseWriter,
	r *http.Request,
) {
	w.Header().Set("Content-Type", "application/json")

	operatorID, ok := r.Context().
		Value(authmiddleware.OperatorIDKey).(string)

	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "operator id not found",
		})
		return
	}

	periodParam := r.URL.Query().Get("period")
	periodDays := 7
	if periodParam == "30d" || periodParam == "30" {
		periodDays = 30
	}

	analytics, err := h.service.GetAnalytics(operatorID, periodDays)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"message": err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"data": analytics,
	})
}
