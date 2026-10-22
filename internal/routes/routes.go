package routes

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	authhandler "ticketdot/internal/auth/handler"
	authmiddleware "ticketdot/internal/auth/middleware"
	bookinghandler "ticketdot/internal/booking/handler"
	"ticketdot/internal/config"
	healthhandler "ticketdot/internal/health"
	httpmiddleware "ticketdot/internal/http/middleware"
	operatorhandler "ticketdot/internal/operator/handler"
	paymenthandler "ticketdot/internal/payment/handler"
	profilehandler "ticketdot/internal/profile/handler"
	reviewhandler "ticketdot/internal/review/handler"
	searchhandler "ticketdot/internal/search/handler"
	seathandler "ticketdot/internal/seat/handler"
	trackinghandler "ticketdot/internal/tracking/handler"
)

type Handlers struct {
	Config   *config.Config
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
	if h.Config != nil && h.Config.TrustProxy {
		r.Use(chimiddleware.RealIP)
	}
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)

	// Baseline security headers
	r.Use(httpmiddleware.SecurityHeaders)

	// Request limits, rate limiting & CORS
	if h.Config != nil {
		r.Use(httpmiddleware.RequestSizeLimiter(h.Config.MaxBodyBytes))
		rl := httpmiddleware.NewRateLimiter(h.Config.LoginRateLimit, h.Config.SignupRateLimit, h.Config.ForgotPasswordRateLimit, h.Config.GlobalRateLimit, h.Config.TrustProxy)
		if h.Config.WebhookRateLimit > 0 {
			rl.SetWebhookRateLimit(h.Config.WebhookRateLimit)
		}
		r.Use(rl.Handler)
		r.Use(httpmiddleware.NewCORSMiddleware(h.Config.AllowedOrigins))
	} else {
		r.Use(httpmiddleware.RequestSizeLimiter(1048576))
		r.Use(httpmiddleware.NewRateLimiter(10, 10, 5, 120, false).Handler)
		r.Use(httpmiddleware.CORS)
	}

	// Liveness & readiness probe
	r.Get("/health", h.Health.Check)

	// Authentication Endpoints
	r.Post("/auth/signup", h.Auth.Signup)
	r.Post("/auth/login", h.Auth.Login)
	r.Post("/auth/refresh", h.Auth.Refresh)
	r.Post("/auth/logout", h.Auth.Logout)
	r.Post("/auth/forgot-password", h.Auth.ForgotPassword)
	r.Post("/auth/reset-password", h.Auth.ResetPassword)

	r.Get("/buses/search", h.Search.SearchBuses)
	r.Get("/buses/{scheduleID}", h.Search.GetScheduleDetails)
	r.Get("/buses/{scheduleID}/seats", h.Seat.GetSeats)
	r.Get("/cities", h.Search.GetCities)

	// Payment Gateway Webhook & Test Signature helper (development only)
	r.Post("/payments/webhook", h.Payment.HandleWebhook)
	if h.Config != nil && h.Config.IsDevelopment() {
		r.Post("/payments/test-signature", h.Payment.GenerateTestSignature)
	} else {
		r.HandleFunc("/payments/test-signature", http.NotFound)
	}

	// Live Bus Tracking Telemetry (Public)
	r.Get("/trips/{identifier}/tracking", h.Tracking.GetTracking)

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
		verifyRateLimit := 10
		if h.Config != nil && h.Config.PaymentVerifyRateLimit > 0 {
			verifyRateLimit = h.Config.PaymentVerifyRateLimit
		}

		r.Post("/payments/orders", h.Payment.CreateOrder)
		r.With(httpmiddleware.NewUserRateLimiter(verifyRateLimit, time.Minute)).Post("/payments/verify", h.Payment.VerifyPayment)
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
		r.Post("/trips/{scheduleID}/simulate", h.Tracking.SimulateTrip)
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
