package repository

import (
	"context"
	"ticketdot/internal/review/model"
)

type ReviewRepository interface {
	CreateReview(ctx context.Context, review *model.Review) error
	GetReviewByBookingRef(ctx context.Context, bookingRef string) (*model.Review, error)
	GetReviewsByBus(ctx context.Context, busID string, limit, offset int) ([]model.Review, error)
	GetReviewsByOperator(ctx context.Context, operatorID string, limit, offset int) ([]model.Review, error)
	GetBusRatingSummary(ctx context.Context, busID string) (*model.BusRatingSummary, error)
	GetBookingTripDetails(ctx context.Context, bookingRef string) (*model.BookingTripDetails, error)
	ReplyToReview(ctx context.Context, reviewID string, operatorID string, reply string) error
}
