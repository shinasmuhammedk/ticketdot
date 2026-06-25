package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	authhandler "ticketdot/internal/auth/handler"
	authmiddleware "ticketdot/internal/auth/middleware"
	bookinghandler "ticketdot/internal/booking/handler"
	httpmiddleware "ticketdot/internal/http/middleware"
	operatorhandler "ticketdot/internal/operator/handler"
	searchhandler "ticketdot/internal/search/handler"
	seathandler "ticketdot/internal/seat/handler"
)

type Handlers struct {
	Auth     *authhandler.AuthHandler
	Search   *searchhandler.SearchHandler
	Seat     *seathandler.SeatHandler
	Booking  *bookinghandler.BookingHandler
	Operator *operatorhandler.OperatorHandler
}

func SetupRoutes(h Handlers) http.Handler {
	r := chi.NewRouter()

	r.Use(httpmiddleware.CORS)

	r.Post("/auth/signup", h.Auth.Signup)
	r.Post("/auth/login", h.Auth.Login)

	r.Get("/buses/search", h.Search.SearchBuses)
	r.Get("/buses/{scheduleID}/seats", h.Seat.GetSeats)

	r.Group(func(r chi.Router) {
		r.Use(authmiddleware.AuthMiddleware)

		r.Get("/auth/me", h.Auth.Me)

		r.Post("/bookings/lock", h.Booking.LockSeat)
		r.Post("/bookings/confirm", h.Booking.ConfirmBooking)

		r.Get("/bookings/my-bookings", h.Booking.GetMyBookings)

		r.Get("/bookings/{bookingRef}", h.Booking.GetBookingDetails)

		r.Post("/bookings/{bookingRef}/cancel", h.Booking.CancelBooking)
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
		r.Get("/operator/bookings", h.Operator.GetBookings)

		r.Get("/operator/buses", h.Operator.GetBuses)
		r.Get("/operator/routes", h.Operator.GetRoutes)
		r.Get("/operator/schedules",h.Operator.GetSchedules)
	})

	return r
}
