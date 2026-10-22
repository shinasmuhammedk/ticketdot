package handler

import (
	"encoding/json"
	"net/http"

	"ticketdot/internal/auth/middleware"
	"ticketdot/internal/auth/service"
	"ticketdot/internal/httpx"
)

type AuthHandler struct {
	service *service.AuthService
}

func NewAuthHandler(service *service.AuthService) *AuthHandler {
	return &AuthHandler{
		service: service,
	}
}

func (h *AuthHandler) Signup(w http.ResponseWriter, r *http.Request) {
	var req service.SignupRequest
	if err := httpx.DecodeJSON(r, &req, true); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	user, err := h.service.Signup(req)
	if err != nil {
		httpx.WriteError(w, r, httpx.Validation(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"message": "signup successful",
		"user":    user,
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req service.LoginRequest
	if err := httpx.DecodeJSON(r, &req, true); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	res, err := h.service.Login(req)
	if err != nil {
		httpx.WriteError(w, r, httpx.Unauthorized(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"message":       "login successful",
		"token":         res.AccessToken,
		"access_token":  res.AccessToken,
		"refresh_token": res.RefreshToken,
		"data":          res,
		"user":          res.User,
	})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req service.RefreshRequest
	if err := httpx.DecodeJSON(r, &req, true); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	res, err := h.service.Refresh(req)
	if err != nil {
		httpx.WriteError(w, r, httpx.Unauthorized(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"message":       "token refreshed successfully",
		"access_token":  res.AccessToken,
		"refresh_token": res.RefreshToken,
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req service.LogoutRequest
	// Logout accepts JSON body if present, but shouldn't error if empty body
	if r.Body != nil && r.ContentLength > 0 {
		if err := httpx.DecodeJSON(r, &req, true); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}

	if err := h.service.Logout(req); err != nil {
		httpx.WriteError(w, r, httpx.Internal(err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "logged out successfully",
	})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		httpx.WriteError(w, r, httpx.Unauthorized("user not found in context"))
		return
	}

	user, err := h.service.Me(userID)
	if err != nil {
		httpx.WriteError(w, r, httpx.NotFound("user not found"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"user": user,
	})
}

func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req service.ForgotPasswordRequest
	if err := httpx.DecodeJSON(r, &req, true); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	res, err := h.service.ForgotPassword(req)
	if err != nil {
		httpx.WriteError(w, r, httpx.Validation(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"message":     res.Message,
		"reset_token": res.ResetToken,
	})
}

func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req service.ResetPasswordRequest
	if err := httpx.DecodeJSON(r, &req, true); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	if err := h.service.ResetPassword(req); err != nil {
		httpx.WriteError(w, r, httpx.Validation(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"message": "Password successfully reset. You can now login with your new password.",
	})
}
