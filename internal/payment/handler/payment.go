package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	authmiddleware "ticketdot/internal/auth/middleware"
	"strings"
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
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req model.CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	res, err := h.service.CreateOrder(r.Context(), userID, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req model.VerifyPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	res, err := h.service.VerifyPayment(r.Context(), userID, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Asynchronously dispatch e-ticket email upon payment verification & seat booking
	if h.emailService != nil && res.ContactEmail != "" {
		go func() {
			defer func() { _ = recover() }()
			var manifest []notification.PassengerManifestItem
			for _, p := range res.Passengers {
				manifest = append(manifest, notification.PassengerManifestItem{
					SeatNumber: p.SeatNumber,
					Name:       p.Name,
					Age:        p.Age,
					Gender:     p.Gender,
				})
			}
			pName := "Passenger"
			if len(res.Passengers) > 0 && res.Passengers[0].Name != "" {
				pName = res.Passengers[0].Name
			}
			emailData := notification.TicketEmailData{
				BookingRef:    res.BookingRef,
				PassengerName: pName,
				Origin:        res.BoardingPoint,
				Destination:   res.DroppingPoint,
				BoardingPoint: res.BoardingPoint,
				DroppingPoint: res.DroppingPoint,
				ContactPhone:  res.ContactPhone,
				ContactEmail:  res.ContactEmail,
				Seats:         res.Seats,
				SeatsDisplay:  strings.Join(res.Seats, ", "),
				Passengers:    manifest,
				Amount:        res.TotalAmount,
				PaymentID:     res.PaymentID,
				PaymentMethod: res.PaymentMethod,
				PaymentStatus: "captured",
			}
			_ = h.emailService.SendTicketEmail(res.ContactEmail, emailData)
		}()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"message": "payment verified and booking confirmed successfully",
		"data":    res,
	})
}

func (h *PaymentHandler) GetPaymentStatus(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")
	if orderID == "" {
		http.Error(w, "order id is required", http.StatusBadRequest)
		return
	}

	res, err := h.service.GetPaymentStatus(r.Context(), orderID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
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
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if body.OrderID == "" || body.PaymentID == "" {
		http.Error(w, "order_id and payment_id are required", http.StatusBadRequest)
		return
	}

	sig := h.service.GenerateSignature(body.OrderID, body.PaymentID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"signature": sig,
		"key_id":    h.service.GetKeyID(),
	})
}

func (h *PaymentHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	signature := r.Header.Get("X-Razorpay-Signature")
	if signature == "" {
		signature = r.Header.Get("x-razorpay-signature")
	}

	if err := h.service.ProcessWebhook(r.Context(), body, signature); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}
