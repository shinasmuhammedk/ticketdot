package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	authhandler "ticketdot/internal/auth/handler"
	authrepository "ticketdot/internal/auth/repository"
	authservice "ticketdot/internal/auth/service"

	"ticketdot/internal/booking/cleanup"
	bookinghandler "ticketdot/internal/booking/handler"
	bookingrepository "ticketdot/internal/booking/repository"
	bookingservice "ticketdot/internal/booking/service"

	"ticketdot/internal/db"
	"ticketdot/internal/health"
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
	"ticketdot/internal/schedule/lifecycle"

	paymenthandler "ticketdot/internal/payment/handler"
	paymentrepository "ticketdot/internal/payment/repository"
	paymentservice "ticketdot/internal/payment/service"

	trackinghandler "ticketdot/internal/tracking/handler"
	trackingrepository "ticketdot/internal/tracking/repository"
	trackingservice "ticketdot/internal/tracking/service"

	reviewhandler "ticketdot/internal/review/handler"
	reviewrepository "ticketdot/internal/review/repository"
	reviewservice "ticketdot/internal/review/service"

	"ticketdot/internal/notification"
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

	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}

	database := db.Connect(databaseURL)

	if err := migrations.Run(database); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cleanup.StartExpiredLockCleaner(ctx, database)
	lifecycle.StartScheduleLifecycleWorker(ctx, database, 30*time.Second)

	healthHandler := health.NewHealthHandler(database)

	authRepo := authrepository.NewPostgresRepository(database)
	authService := authservice.NewAuthService(authRepo)
	authHandler := authhandler.NewAuthHandler(authService)

	searchRepo := searchrepository.NewPostgresRepository(database)
	searchService := searchservice.NewSearchService(searchRepo)
	searchHandler := searchhandler.NewSearchHandler(searchService)

	seatRepo := seatrepository.NewPostgresRepository(database)
	seatService := seatservice.NewSeatService(seatRepo)
	seatHandler := seathandler.NewSeatHandler(seatService)

	emailService := notification.NewEmailService()

	bookingRepo := bookingrepository.NewPostgresRepository(database)
	bookingService := bookingservice.NewBookingService(bookingRepo)
	bookingHandler := bookinghandler.NewBookingHandler(bookingService, emailService)

	operatorRepo := operatorrepository.NewPostgresRepository(database)
	operatorService := operatorservice.NewOperatorService(operatorRepo)
	operatorHandler := operatorhandler.NewOperatorHandler(operatorService)

	profileRepo := profilerepository.NewPostgresRepository(database)
	profileService := profileservice.NewProfileService(profileRepo)
	profileHandler := profilehandler.NewProfileHandler(profileService)

	paymentRepo := paymentrepository.NewPostgresRepository(database)
	paymentService := paymentservice.NewPaymentService(paymentRepo)
	paymentHandler := paymenthandler.NewPaymentHandler(paymentService, emailService)

	trackingRepo := trackingrepository.NewPostgresTrackingRepository(database)
	trackingService := trackingservice.NewTrackingService(trackingRepo)
	trackingHandler := trackinghandler.NewTrackingHandler(trackingService)

	reviewRepo := reviewrepository.NewPostgresReviewRepository(database)
	reviewService := reviewservice.NewReviewService(reviewRepo)
	reviewHandler := reviewhandler.NewReviewHandler(reviewService)

	router := routes.SetupRoutes(routes.Handlers{
		Health:   healthHandler,
		Auth:     authHandler,
		Search:   searchHandler,
		Seat:     seatHandler,
		Booking:  bookingHandler,
		Operator: operatorHandler,
		Profile:  profileHandler,
		Payment:  paymentHandler,
		Tracking: trackingHandler,
		Review:   reviewHandler,
	})

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Run HTTP listener in goroutine
	go func() {
		log.Printf("Server running on :%s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server gracefully...")
	cancel() // Cancel background lock cleaner

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited cleanly")
}

