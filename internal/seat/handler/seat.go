package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"ticketdot/internal/seat/service"
)

type SeatHandler struct {
	service *service.SeatService
}

func NewSeatHandler(service *service.SeatService) *SeatHandler {
	return &SeatHandler{
		service: service,
	}
}

func (h *SeatHandler) GetSeats(w http.ResponseWriter, r *http.Request) {
	scheduleID := chi.URLParam(r, "scheduleID")

	seats, err := h.service.GetSeatsByScheduleID(scheduleID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	response := map[string]any{
		"schedule_id": scheduleID,
		"count":       len(seats),
		"data":        seats,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
