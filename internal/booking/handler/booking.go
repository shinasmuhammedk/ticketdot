package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	authmiddleware "ticketdot/internal/auth/middleware"
	"ticketdot/internal/booking/model"
	"ticketdot/internal/booking/service"
	"ticketdot/internal/httpx"
	"ticketdot/internal/notification"
)

type BookingHandler struct {
	service      *service.BookingService
	emailService *notification.EmailService
}

func NewBookingHandler(service *service.BookingService, emailService *notification.EmailService) *BookingHandler {
	return &BookingHandler{
		service:      service,
		emailService: emailService,
	}
}

func (h *BookingHandler) LockSeat(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(authmiddleware.UserIDKey).(string)
	if !ok || userID == "" {
		httpx.WriteError(w, r, httpx.Unauthorized("unauthorized"))
		return
	}

	var req model.LockSeatRequest
	if err := httpx.DecodeJSON(r, &req, false); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	res, err := h.service.LockSeat(userID, req)
	if err != nil {
		httpx.WriteError(w, r, httpx.Validation(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"message": "seats locked successfully",
		"data":    res,
	})
}

func (h *BookingHandler) ConfirmBooking(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(authmiddleware.UserIDKey).(string)
	if !ok || userID == "" {
		httpx.WriteError(w, r, httpx.Unauthorized("unauthorized"))
		return
	}

	var req model.ConfirmBookingRequest
	if err := httpx.DecodeJSON(r, &req, false); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	res, err := h.service.ConfirmBooking(userID, req)
	if err != nil {
		httpx.WriteError(w, r, httpx.Validation(err.Error()))
		return
	}

	// Asynchronously dispatch e-ticket email if recipient address provided
	if h.emailService != nil && res.ContactEmail != "" {
		h.dispatchTicketEmailAsync(userID, res.BookingRef, res.ContactEmail)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(map[string]any{
		"message": "booking confirmed",
		"data":    res,
	})
}

func (h *BookingHandler) ReleaseSeats(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(authmiddleware.UserIDKey).(string)
	if !ok || userID == "" {
		httpx.WriteError(w, r, httpx.Unauthorized("unauthorized"))
		return
	}

	var req model.ReleaseSeatsRequest
	if err := httpx.DecodeJSON(r, &req, false); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	res, err := h.service.ReleaseSeats(userID, req)
	if err != nil {
		httpx.WriteError(w, r, httpx.Validation(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(map[string]any{
		"message": res.Message,
		"data":    res,
	})
}

func (h *BookingHandler) GetMyBookings(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(authmiddleware.UserIDKey).(string)
	if !ok || userID == "" {
		httpx.WriteError(w, r, httpx.Unauthorized("unauthorized"))
		return
	}

	bookings, err := h.service.GetMyBookings(userID)
	if err != nil {
		httpx.WriteError(w, r, httpx.Validation(err.Error()))
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
	if !ok || userID == "" {
		httpx.WriteError(w, r, httpx.Unauthorized("unauthorized"))
		return
	}

	bookingRef := chi.URLParam(r, "bookingRef")

	res, err := h.service.CancelBooking(userID, bookingRef)
	if err != nil {
		httpx.WriteError(w, r, httpx.Validation(err.Error()))
		return
	}

	// Asynchronously dispatch cancellation/refund notification email
	if h.emailService != nil {
		h.dispatchRefundEmailAsync(userID, bookingRef, res)
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
	if !ok || userID == "" {
		httpx.WriteError(w, r, httpx.Unauthorized("unauthorized"))
		return
	}

	bookingRef := chi.URLParam(r, "bookingRef")

	res, err := h.service.GetBookingDetails(
		userID,
		bookingRef,
	)

	if err != nil {
		httpx.WriteError(w, r, httpx.NotFound("booking not found"))
		return
	}

	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(map[string]any{
		"data": res,
	})
}

// SendTicketEmail allows on-demand email dispatch of the e-ticket
func (h *BookingHandler) SendTicketEmail(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(authmiddleware.UserIDKey).(string)
	if !ok || userID == "" {
		httpx.WriteError(w, r, httpx.Unauthorized("unauthorized"))
		return
	}

	bookingRef := chi.URLParam(r, "bookingRef")
	var req struct {
		Email string `json:"email"`
	}
	_ = httpx.DecodeJSON(r, &req, false)

	details, err := h.service.GetBookingDetails(userID, bookingRef)
	if err != nil {
		httpx.WriteError(w, r, httpx.NotFound("booking not found"))
		return
	}

	recipient := strings.TrimSpace(req.Email)
	if recipient == "" {
		recipient = details.ContactEmail
	}
	if recipient == "" {
		httpx.WriteError(w, r, httpx.Validation("recipient email is required"))
		return
	}

	var manifest []notification.PassengerManifestItem
	for _, p := range details.Passengers {
		manifest = append(manifest, notification.PassengerManifestItem{
			SeatNumber: p.SeatNumber,
			Name:       p.Name,
			Age:        p.Age,
			Gender:     p.Gender,
		})
	}

	emailData := notification.TicketEmailData{
		BookingRef:    details.BookingRef,
		PassengerName: details.PassengerName,
		OperatorName:  details.OperatorName,
		Origin:        details.Origin,
		Destination:   details.Destination,
		DepartureTime: details.DepartureTime,
		ArrivalTime:   details.ArrivalTime,
		BusNumber:     details.BusNumber,
		BusType:       details.BusType,
		BoardingPoint: details.BoardingPoint,
		DroppingPoint: details.DroppingPoint,
		ContactPhone:  details.ContactPhone,
		ContactEmail:  details.ContactEmail,
		Seats:         details.Seats,
		SeatsDisplay:  details.SeatNumber,
		Passengers:    manifest,
		Amount:        details.Amount,
		PaymentID:     details.PaymentID,
		PaymentMethod: details.PaymentMethod,
		PaymentStatus: details.PaymentStatus,
	}

	if err := h.emailService.SendTicketEmail(recipient, emailData); err != nil {
		httpx.WriteError(w, r, httpx.Internal(err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"message":   fmt.Sprintf("e-ticket successfully sent to %s", recipient),
		"recipient": recipient,
	})
}

func (h *BookingHandler) dispatchTicketEmailAsync(userID, bookingRef, recipientEmail string) {
	if h.emailService == nil {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[EMAIL PANIC RECOVERED] %v", r)
			}
		}()
		details, err := h.service.GetBookingDetails(userID, bookingRef)
		if err != nil {
			return
		}
		if recipientEmail == "" {
			recipientEmail = details.ContactEmail
		}
		if recipientEmail == "" {
			return
		}

		var manifest []notification.PassengerManifestItem
		for _, p := range details.Passengers {
			manifest = append(manifest, notification.PassengerManifestItem{
				SeatNumber: p.SeatNumber,
				Name:       p.Name,
				Age:        p.Age,
				Gender:     p.Gender,
			})
		}

		emailData := notification.TicketEmailData{
			BookingRef:    details.BookingRef,
			PassengerName: details.PassengerName,
			OperatorName:  details.OperatorName,
			Origin:        details.Origin,
			Destination:   details.Destination,
			DepartureTime: details.DepartureTime,
			ArrivalTime:   details.ArrivalTime,
			BusNumber:     details.BusNumber,
			BusType:       details.BusType,
			BoardingPoint: details.BoardingPoint,
			DroppingPoint: details.DroppingPoint,
			ContactPhone:  details.ContactPhone,
			ContactEmail:  details.ContactEmail,
			Seats:         details.Seats,
			SeatsDisplay:  details.SeatNumber,
			Passengers:    manifest,
			Amount:        details.Amount,
			PaymentID:     details.PaymentID,
			PaymentMethod: details.PaymentMethod,
			PaymentStatus: details.PaymentStatus,
		}

		_ = h.emailService.SendTicketEmail(recipientEmail, emailData)
	}()
}

func (h *BookingHandler) dispatchRefundEmailAsync(userID, bookingRef string, res *model.CancelBookingResponse) {
	if h.emailService == nil {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[EMAIL PANIC RECOVERED] %v", r)
			}
		}()
		details, err := h.service.GetBookingDetails(userID, bookingRef)
		if err != nil || details.ContactEmail == "" {
			return
		}

		refundData := notification.RefundEmailData{
			BookingRef:     details.BookingRef,
			PassengerName:  details.PassengerName,
			OperatorName:   details.OperatorName,
			Origin:         details.Origin,
			Destination:    details.Destination,
			SeatsDisplay:   details.SeatNumber,
			RefundID:       res.RefundID,
			RefundAmount:   res.RefundAmount,
			RefundStatus:   res.RefundStatus,
			RefundTimeline: res.RefundTimeline,
		}

		_ = h.emailService.SendRefundEmail(details.ContactEmail, refundData)
	}()
}
