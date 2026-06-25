package handler

import (
	"encoding/json"
	"net/http"

	authmiddleware "ticketdot/internal/auth/middleware"
	"ticketdot/internal/booking/model"
	"ticketdot/internal/booking/service"

	"github.com/go-chi/chi/v5"
)

type BookingHandler struct {
	service *service.BookingService
}

func NewBookingHandler(service *service.BookingService) *BookingHandler {
	return &BookingHandler{
		service: service,
	}
}

func (h *BookingHandler) LockSeat(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(authmiddleware.UserIDKey).(string)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req model.LockSeatRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	res, err := h.service.LockSeat(userID, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(map[string]any{
		"message": "seat locked successfully",
		"data":    res,
	})
}

func (h *BookingHandler) ConfirmBooking(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(authmiddleware.UserIDKey).(string)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req model.ConfirmBookingRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	res, err := h.service.ConfirmBooking(userID, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(map[string]any{
		"message": "booking confirmed",
		"data":    res,
	})
}

func (h *BookingHandler) GetMyBookings(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(authmiddleware.UserIDKey).(string)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	bookings, err := h.service.GetMyBookings(userID)
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

func (h *BookingHandler) CancelBooking(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(authmiddleware.UserIDKey).(string)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	bookingRef := chi.URLParam(r, "bookingRef")

	res, err := h.service.CancelBooking(userID, bookingRef)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(map[string]any{
		"message": "booking cancelled successfully",
		"data":    res,
	})
}

func (h *BookingHandler) GetBookingDetails(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := r.Context().Value(authmiddleware.UserIDKey).(string)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	bookingRef := chi.URLParam(r, "bookingRef")

	res, err := h.service.GetBookingDetails(
		userID,
		bookingRef,
	)

	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(map[string]any{
		"data": res,
	})
}
