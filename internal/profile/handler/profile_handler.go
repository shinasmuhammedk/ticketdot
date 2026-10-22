package handler

import (
	"encoding/json"
	"net/http"

	authmiddleware "ticketdot/internal/auth/middleware"
	"ticketdot/internal/httpx"
	"ticketdot/internal/profile/model"
	"ticketdot/internal/profile/service"
)

type ProfileHandler struct {
	service *service.ProfileService
}

func NewProfileHandler(
	service *service.ProfileService,
) *ProfileHandler {
	return &ProfileHandler{
		service: service,
	}
}

func (h *ProfileHandler) GetProfile(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := r.Context().Value(authmiddleware.UserIDKey).(string)
	if !ok || userID == "" {
		httpx.WriteError(w, r, httpx.Unauthorized("unauthorized"))
		return
	}

	profile, err := h.service.GetProfile(userID)
	if err != nil {
		httpx.WriteError(w, r, httpx.Internal(err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": profile,
	})
}

func (h *ProfileHandler) UpdateProfile(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := r.Context().Value(authmiddleware.UserIDKey).(string)
	if !ok || userID == "" {
		httpx.WriteError(w, r, httpx.Unauthorized("unauthorized"))
		return
	}

	var req model.UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.Validation("invalid request body"))
		return
	}

	if err := h.service.UpdateProfile(userID, &req); err != nil {
		httpx.WriteError(w, r, httpx.Validation(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"message": "Profile updated successfully",
	})
}
