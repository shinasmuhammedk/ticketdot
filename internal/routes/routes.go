package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	authhandler "ticketdot/internal/auth/handler"
	authmiddleware "ticketdot/internal/auth/middleware"
	bookinghandler "ticketdot/internal/booking/handler"
	healthhandler "ticketdot/internal/health"
	httpmiddleware "ticketdot/internal/http/middleware"
	operatorhandler "ticketdot/internal/operator/handler"
	paymenthandler "ticketdot/internal/payment/handler"
	profilehandler "ticketdot/internal/profile/handler"
	searchhandler "ticketdot/internal/search/handler"
	seathandler "ticketdot/internal/seat/handler"
	trackinghandler "ticketdot/internal/tracking/handler"
	reviewhandler "ticketdot/internal/review/handler"
)

type Handlers struct {
	Health   *healthhandler.HealthHandler
	Auth     *authhandler.AuthHandler
	Search   *searchhandler.SearchHandler
	Seat     *seathandler.SeatHandler
	Booking  *bookinghandler.BookingHandler
	Operator *operatorhandler.OperatorHandler
	Profile  *profilehandler.ProfileHandler
	Payment  *paymenthandler.PaymentHandler
	Tracking *trackinghandler.TrackingHandler
	Review   *reviewhandler.ReviewHandler
}

func SetupRoutes(h Handlers) http.Handler {
	r := chi.NewRouter()

	// Production resilience & observability middlewares
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(httpmiddleware.CORS)

	// Liveness & readiness probe
	r.Get("/health", h.Health.Check)

	r.Post("/auth/signup", h.Auth.Signup)
	r.Post("/auth/login", h.Auth.Login)
	r.Post("/auth/forgot-password", h.Auth.ForgotPassword)
	r.Post("/auth/reset-password", h.Auth.ResetPassword)

	r.Get("/buses/search", h.Search.SearchBuses)
	r.Get("/buses/{scheduleID}", h.Search.GetScheduleDetails)
	r.Get("/buses/{scheduleID}/seats", h.Seat.GetSeats)
	r.Get("/cities", h.Search.GetCities)

	// Payment Gateway Webhook & Test Signature helper
	r.Post("/payments/webhook", h.Payment.HandleWebhook)
	r.Post("/payments/test-signature", h.Payment.GenerateTestSignature)

	// Live Bus Tracking & Trip Telemetry
	r.Get("/trips/{identifier}/tracking", h.Tracking.GetTracking)
	r.Post("/trips/{scheduleID}/simulate", h.Tracking.SimulateTrip)

	// Bus Reviews & Passenger Ratings
	r.Get("/buses/{busID}/reviews", h.Review.GetBusReviews)
	r.Get("/reviews/booking/{bookingRef}", h.Review.GetBookingReview)

	r.Group(func(r chi.Router) {
		r.Use(authmiddleware.AuthMiddleware)

		// Passenger Reviews
		r.Post("/reviews", h.Review.SubmitReview)

		r.Get("/auth/me", h.Auth.Me)

		// Profile
		r.Get("/profile", h.Profile.GetProfile)
		r.Put("/profile", h.Profile.UpdateProfile)

		// Booking Lifecycle
		r.Post("/bookings/lock", h.Booking.LockSeat)
		r.Post("/bookings/confirm", h.Booking.ConfirmBooking)
		r.Post("/bookings/release", h.Booking.ReleaseSeats)

		r.Get("/bookings/my-bookings", h.Booking.GetMyBookings)
		r.Get("/bookings/{bookingRef}", h.Booking.GetBookingDetails)
		r.Post("/bookings/{bookingRef}/email", h.Booking.SendTicketEmail)
		r.Post("/bookings/{bookingRef}/cancel", h.Booking.CancelBooking)

		// Payment Gateway Endpoints
		r.Post("/payments/orders", h.Payment.CreateOrder)
		r.Post("/payments/verify", h.Payment.VerifyPayment)
		r.Get("/payments/{orderID}", h.Payment.GetPaymentStatus)
	})

	r.Group(func(r chi.Router) {
		r.Use(authmiddleware.AuthMiddleware)
		r.Use(authmiddleware.RequireRole("passenger"))

		r.Get("/passenger/dashboard", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("passenger dashboard"))
		})
	})

	r.Group(func(r chi.Router) {
		r.Use(authmiddleware.AuthMiddleware)
		r.Use(authmiddleware.RequireRole("operator_admin"))

		r.Post("/operator/buses", h.Operator.CreateBus)
		r.Post("/operator/routes", h.Operator.CreateRoute)
		r.Post("/operator/schedules", h.Operator.CreateSchedule)
		r.Post("/operator/schedules/{scheduleID}/cancel", h.Operator.CancelSchedule)
		r.Post("/operator/trips/{scheduleID}/location", h.Tracking.UpdateLocation)
		r.Get("/operator/bookings", h.Operator.GetBookings)

		r.Get("/operator/buses", h.Operator.GetBuses)
		r.Get("/operator/buses/{busID}/layout", h.Operator.GetBusLayout)
		r.Get("/operator/routes", h.Operator.GetRoutes)
		r.Get("/operator/schedules", h.Operator.GetSchedules)
		r.Get("/operator/dashboard", h.Operator.GetDashboardStats)
		r.Get("/operator/analytics", h.Operator.GetAnalytics)

		r.Put("/operator/buses/{busID}", h.Operator.UpdateBus)
		r.Delete("/operator/buses/{busID}", h.Operator.DeleteBus)

		// Operator Reviews & Passenger Feedback
		r.Get("/operator/reviews", h.Review.GetOperatorReviews)
		r.Post("/operator/reviews/{reviewID}/reply", h.Review.ReplyToReview)
	})

	return r
}
