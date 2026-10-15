package handler

import (
	"encoding/json"
	"net/http"

	authmiddleware "ticketdot/internal/auth/middleware"
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

	userID, ok := r.Context().
		Value(authmiddleware.UserIDKey).(string)

	if !ok {
		http.Error(
			w,
			"unauthorized",
			http.StatusUnauthorized,
		)
		return
	}

	profile, err := h.service.GetProfile(userID)

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
			"data": profile,
		},
	)
}

func (h *ProfileHandler) UpdateProfile(
	w http.ResponseWriter,
	r *http.Request,
) {

	userID, ok := r.Context().
		Value(authmiddleware.UserIDKey).(string)

	if !ok {
		http.Error(
			w,
			"unauthorized",
			http.StatusUnauthorized,
		)
		return
	}

	var req model.UpdateProfileRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(
			w,
			"invalid request body",
			http.StatusBadRequest,
		)
		return
	}

	if err := h.service.UpdateProfile(
		userID,
		&req,
	); err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusBadRequest,
		)
		return
	}

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	json.NewEncoder(w).Encode(map[string]any{
		"message": "Profile updated successfully",
	})
}
