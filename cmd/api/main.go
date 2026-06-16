package main

import (
	"log"
	"net/http"
	"os"

	"context"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"

	authhandler "ticketdot/internal/auth/handler"
	"ticketdot/internal/auth/middleware"
	authrepository "ticketdot/internal/auth/repository"
	authservice "ticketdot/internal/auth/service"

	searchhandler "ticketdot/internal/search/handler"
	searchrepository "ticketdot/internal/search/repository"
	searchservice "ticketdot/internal/search/service"

	"ticketdot/internal/db"

	seathandler "ticketdot/internal/seat/handler"
	seatrepository "ticketdot/internal/seat/repository"
	seatservice "ticketdot/internal/seat/service"

	"ticketdot/internal/booking/cleanup"
	bookinghandler "ticketdot/internal/booking/handler"
	bookingrepository "ticketdot/internal/booking/repository"
	bookingservice "ticketdot/internal/booking/service"

	operatorhandler "ticketdot/internal/operator/handler"
	operatorrepository "ticketdot/internal/operator/repository"
	operatorservice "ticketdot/internal/operator/service"
)

func main() {
	// Connect Database
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	log.Println("DATABASE_URL:", databaseURL)

	database := db.Connect(databaseURL)

	ctx := context.Background()
	cleanup.StartExpiredLockCleaner(ctx, database)

	// Dependency Injection
	authRepo := authrepository.NewPostgresRepository(database)
	authService := authservice.NewAuthService(authRepo)
	authHandler := authhandler.NewAuthHandler(authService)

	searchRepo := searchrepository.NewPostgresRepository(database)
	searchService := searchservice.NewSearchService(searchRepo)
	searchHandler := searchhandler.NewSearchHandler(searchService)

	seatRepo := seatrepository.NewPostgresRepository(database)
	seatService := seatservice.NewSeatService(seatRepo)
	seatHandler := seathandler.NewSeatHandler(seatService)

	bookingRepo := bookingrepository.NewPostgresRepository(database)
	bookingService := bookingservice.NewBookingService(bookingRepo)
	bookingHandler := bookinghandler.NewBookingHandler(bookingService)

	operatorRepo := operatorrepository.NewPostgresRepository(database)
	operatorService := operatorservice.NewOperatorService(operatorRepo)
	operatorHandler := operatorhandler.NewOperatorHandler(operatorService)

	// Router
	r := chi.NewRouter()

	r.Post("/auth/signup", authHandler.Signup)
	r.Post("/auth/login", authHandler.Login)

	r.Get("/buses/search", searchHandler.SearchBuses)

	r.Get("/buses/{scheduleID}/seats", seatHandler.GetSeats)

	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthMiddleware)

		r.Get("/auth/me", authHandler.Me)
	})

	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthMiddleware)
		r.Use(middleware.RequireRole("passenger"))

		r.Get("/passenger/dashboard", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("passenger dashboard"))
		})
	})

	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthMiddleware)

		r.Post("/bookings/lock", bookingHandler.LockSeat)
		r.Post("/bookings/confirm", bookingHandler.ConfirmBooking)
		r.Get("/bookings/my-bookings", bookingHandler.GetMyBookings)
		r.Post("/bookings/{bookingRef}/cancel", bookingHandler.CancelBooking)
	})

	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthMiddleware)
		r.Use(middleware.RequireRole("operator_admin"))

		r.Post("/operator/buses", operatorHandler.CreateBus)
        r.Post("/operator/routes", operatorHandler.CreateRoute)
        r.Post("/operator/schedules", operatorHandler.CreateSchedule)
        
        r.Get("/operator/bookings", operatorHandler.GetBookings)
	})

	log.Println("Server running on :8080")

	http.ListenAndServe(":8080", r)
}
