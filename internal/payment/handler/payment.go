package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	authmiddleware "ticketdot/internal/auth/middleware"
	"ticketdot/internal/httpx"
	"ticketdot/internal/notification"
	"ticketdot/internal/payment/model"
	"ticketdot/internal/payment/service"
)

type PaymentHandler struct {
	service      *service.PaymentService
	emailService *notification.EmailService
}

func NewPaymentHandler(service *service.PaymentService, emailService *notification.EmailService) *PaymentHandler {
	return &PaymentHandler{
		service:      service,
		emailService: emailService,
	}
}

func (h *PaymentHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(authmiddleware.UserIDKey).(string)
	if !ok || userID == "" {
		httpx.WriteError(w, r, httpx.Unauthorized("unauthorized"))
		return
	}

	var req model.CreateOrderRequest
	if err := httpx.DecodeJSON(r, &req, true); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	res, err := h.service.CreateOrder(r.Context(), userID, req)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"message": "payment order created successfully",
		"data":    res,
	})
}

func (h *PaymentHandler) VerifyPayment(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(authmiddleware.UserIDKey).(string)
	if !ok || userID == "" {
		httpx.WriteError(w, r, httpx.Unauthorized("unauthorized"))
		return
	}

	var req model.VerifyPaymentRequest
	if err := httpx.DecodeJSON(r, &req, true); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	res, err := h.service.VerifyPayment(r.Context(), userID, req)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	if res.Status == "processing" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]any{
			"status":  "processing",
			"message": "payment authorized and being processed",
			"data":    res,
		})
		return
	}

	if res.Status == "refund_required" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"status":  "refund_required",
			"message": "payment received, booking could not be completed, refund will be processed",
			"data":    res,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"message": "payment verified and booking confirmed successfully",
		"data":    res,
	})
}

func (h *PaymentHandler) GetPaymentStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(authmiddleware.UserIDKey).(string)
	if !ok || userID == "" {
		httpx.WriteError(w, r, httpx.Unauthorized("unauthorized"))
		return
	}

	orderID := chi.URLParam(r, "orderID")
	if orderID == "" {
		httpx.WriteError(w, r, httpx.Validation("order id is required"))
		return
	}

	res, err := h.service.GetPaymentStatus(r.Context(), userID, orderID)
	if err != nil {
		httpx.WriteError(w, r, httpx.NotFound("payment not found"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": res,
	})
}

// GenerateTestSignature provides a test-mode helper to compute valid HMAC signatures
// without leaking the gateway secret to the frontend.
func (h *PaymentHandler) GenerateTestSignature(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OrderID   string `json:"order_id"`
		PaymentID string `json:"payment_id"`
	}
	if err := httpx.DecodeJSON(r, &body, true); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	if body.OrderID == "" || body.PaymentID == "" {
		httpx.WriteError(w, r, httpx.Validation("order_id and payment_id are required"))
		return
	}

	_ = h.service.RegisterTestPaymentIfFake(r.Context(), body.OrderID, body.PaymentID)

	sig := h.service.GenerateSignature(body.OrderID, body.PaymentID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"signature": sig,
		"key_id":    h.service.GetKeyID(),
	})
}

func (h *PaymentHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	// 1. Read raw body through MaxBytesReader (default 1 MB)
	const maxWebhookBodyBytes = int64(1048576) // 1 MB
	r.Body = http.MaxBytesReader(w, r.Body, maxWebhookBodyBytes)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "request entity too large"})
			return
		}
		httpx.WriteError(w, r, httpx.Validation("failed to read body"))
		return
	}
	defer r.Body.Close()

	// 2. Validate X-Razorpay-Signature header
	signature := strings.TrimSpace(r.Header.Get("X-Razorpay-Signature"))
	if signature == "" {
		signature = strings.TrimSpace(r.Header.Get("x-razorpay-signature"))
	}
	if signature == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	eventID := strings.TrimSpace(r.Header.Get("X-Razorpay-Event-Id"))
	if eventID == "" {
		eventID = strings.TrimSpace(r.Header.Get("x-razorpay-event-id"))
	}

	// 3. Process webhook event atomically
	if err := h.service.ProcessWebhook(r.Context(), eventID, body, signature); err != nil {
		var appErr *httpx.AppError
		if errors.As(err, &appErr) {
			if appErr.StatusCode == http.StatusUnauthorized {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
				return
			}
			httpx.WriteError(w, r, appErr)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}
