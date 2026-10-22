package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"ticketdot/internal/httpx"
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
		httpx.WriteError(w, r, httpx.Validation(err.Error()))
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
			httpx.WriteError(w, r, httpx.NotFound("schedule not found"))
			return
		}
		httpx.WriteError(w, r, httpx.Internal(err))
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
		httpx.WriteError(w, r, httpx.Internal(err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"count": len(cities),
		"data":  cities,
	})
}
