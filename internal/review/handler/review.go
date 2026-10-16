package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	authmiddleware "ticketdot/internal/auth/middleware"
	"ticketdot/internal/review/model"
	"ticketdot/internal/review/service"

	"github.com/go-chi/chi/v5"
)

type ReviewHandler struct {
	service *service.ReviewService
}

func NewReviewHandler(service *service.ReviewService) *ReviewHandler {
	return &ReviewHandler{service: service}
}

// SubmitReview handles passenger submission of verified trip feedback
func (h *ReviewHandler) SubmitReview(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(authmiddleware.UserIDKey).(string)
	if !ok || userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req model.SubmitReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	review, err := h.service.SubmitReview(r.Context(), userID, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"message": "review submitted successfully",
		"data":    review,
	})
}

// GetBookingReview checks if a review exists for a given booking reference
func (h *ReviewHandler) GetBookingReview(w http.ResponseWriter, r *http.Request) {
	bookingRef := chi.URLParam(r, "bookingRef")
	if bookingRef == "" {
		http.Error(w, "missing booking reference", http.StatusBadRequest)
		return
	}

	review, err := h.service.GetBookingReview(r.Context(), bookingRef)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": review,
	})
}

// GetBusReviews retrieves reviews & ratings breakdown for a bus
func (h *ReviewHandler) GetBusReviews(w http.ResponseWriter, r *http.Request) {
	busID := chi.URLParam(r, "busID")
	if busID == "" {
		http.Error(w, "missing bus ID", http.StatusBadRequest)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	res, err := h.service.GetBusReviews(r.Context(), busID, page, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": res,
	})
}

// GetOperatorReviews retrieves reviews for the authenticated operator
func (h *ReviewHandler) GetOperatorReviews(w http.ResponseWriter, r *http.Request) {
	operatorID, _ := r.Context().Value(authmiddleware.OperatorIDKey).(string)
	if operatorID == "" {
		operatorID, _ = r.Context().Value(authmiddleware.UserIDKey).(string)
	}
	if operatorID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	res, err := h.service.GetOperatorReviews(r.Context(), operatorID, page, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": res,
	})
}

// ReplyToReview allows the operator to submit a response
func (h *ReviewHandler) ReplyToReview(w http.ResponseWriter, r *http.Request) {
	operatorID, _ := r.Context().Value(authmiddleware.OperatorIDKey).(string)
	if operatorID == "" {
		operatorID, _ = r.Context().Value(authmiddleware.UserIDKey).(string)
	}
	if operatorID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	reviewID := chi.URLParam(r, "reviewID")
	if reviewID == "" {
		http.Error(w, "missing review ID", http.StatusBadRequest)
		return
	}

	var req model.OperatorReplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.service.ReplyToReview(r.Context(), reviewID, operatorID, req.Reply); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"message": "operator reply posted successfully",
	})
}
