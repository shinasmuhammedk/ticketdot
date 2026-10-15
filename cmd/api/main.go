package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"

	authhandler "ticketdot/internal/auth/handler"
	authrepository "ticketdot/internal/auth/repository"
	authservice "ticketdot/internal/auth/service"

	"ticketdot/internal/booking/cleanup"
	bookinghandler "ticketdot/internal/booking/handler"
	bookingrepository "ticketdot/internal/booking/repository"
	bookingservice "ticketdot/internal/booking/service"

	"ticketdot/internal/db"
	"ticketdot/migrations"

	operatorhandler "ticketdot/internal/operator/handler"
	operatorrepository "ticketdot/internal/operator/repository"
	operatorservice "ticketdot/internal/operator/service"

	"ticketdot/internal/routes"

	searchhandler "ticketdot/internal/search/handler"
	searchrepository "ticketdot/internal/search/repository"
	searchservice "ticketdot/internal/search/service"

	seathandler "ticketdot/internal/seat/handler"
	seatrepository "ticketdot/internal/seat/repository"
	seatservice "ticketdot/internal/seat/service"

	profilehandler "ticketdot/internal/profile/handler"
	profilerepository "ticketdot/internal/profile/repository"
	profileservice "ticketdot/internal/profile/service"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	jwtSecret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET is not set")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	database := db.Connect(databaseURL)

	if err := migrations.Run(database); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	ctx := context.Background()
	cleanup.StartExpiredLockCleaner(ctx, database)

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

	profileRepo := profilerepository.NewPostgresRepository(database)
	profileService := profileservice.NewProfileService(profileRepo)
	profileHandler := profilehandler.NewProfileHandler(profileService)

	router := routes.SetupRoutes(routes.Handlers{
		Auth:     authHandler,
		Search:   searchHandler,
		Seat:     seatHandler,
		Booking:  bookingHandler,
		Operator: operatorHandler,
		Profile:  profileHandler,
	})

	log.Println("Server running on :8080")

	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatal(err)
	}
}
