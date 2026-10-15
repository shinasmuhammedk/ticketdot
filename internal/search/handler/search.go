package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"ticketdot/internal/search/service"

	"github.com/go-chi/chi/v5"
)

type SearchHandler struct {
	service *service.SearchService
}

func NewSearchHandler(service *service.SearchService) *SearchHandler {
	return &SearchHandler{
		service: service,
	}
}

func (h *SearchHandler) SearchBuses(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	date := r.URL.Query().Get("date")

	buses, err := h.service.SearchBuses(from, to, date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	response := map[string]any{
		"count": len(buses),
		"data":  buses,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *SearchHandler) GetScheduleDetails(w http.ResponseWriter, r *http.Request) {
	scheduleID := chi.URLParam(r, "scheduleID")

	schedule, err := h.service.GetScheduleDetails(scheduleID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "schedule not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": schedule,
	})
}

func (h *SearchHandler) GetCities(w http.ResponseWriter, r *http.Request) {
	cities, err := h.service.GetCities()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"count": len(cities),
		"data":  cities,
	})
}

