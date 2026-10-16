package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"ticketdot/internal/review/model"
	"ticketdot/internal/review/repository"
)

type ReviewService struct {
	repo repository.ReviewRepository
}

func NewReviewService(repo repository.ReviewRepository) *ReviewService {
	return &ReviewService{repo: repo}
}

// SubmitReview processes and stores a verified passenger review
func (s *ReviewService) SubmitReview(ctx context.Context, userID string, req model.SubmitReviewRequest) (*model.Review, error) {
	req.BookingRef = strings.TrimSpace(req.BookingRef)
	if req.BookingRef == "" {
		return nil, errors.New("booking reference is required")
	}

	// Validate star ratings range (1 to 5)
	if req.CleanlinessRating < 1 || req.CleanlinessRating > 5 ||
		req.PunctualityRating < 1 || req.PunctualityRating > 5 ||
		req.ComfortRating < 1 || req.ComfortRating > 5 ||
		req.StaffRating < 1 || req.StaffRating > 5 {
		return nil, errors.New("all category ratings must be between 1 and 5 stars")
	}

	tripDetails, err := s.repo.GetBookingTripDetails(ctx, req.BookingRef)
	if err != nil {
		return nil, fmt.Errorf("failed to verify booking: %w", err)
	}
	if tripDetails == nil {
		return nil, errors.New("booking not found")
	}

	if tripDetails.UserID != userID {
		return nil, errors.New("unauthorized: review can only be submitted by the passenger who booked this trip")
	}

	if tripDetails.Status == "cancelled" {
		return nil, errors.New("cannot review a cancelled booking")
	}

	// Verify that this booking has not already been reviewed
	existing, err := s.repo.GetReviewByBookingRef(ctx, req.BookingRef)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing reviews: %w", err)
	}
	if existing != nil {
		return nil, errors.New("a verified review has already been submitted for this booking")
	}

	// Compute overall weighted average rating
	avg := float64(req.CleanlinessRating+req.PunctualityRating+req.ComfortRating+req.StaffRating) / 4.0
	overallRating := math.Round(avg*10) / 10

	headline := strings.TrimSpace(req.Headline)
	if headline == "" {
		headline = fmt.Sprintf("Journey Review — %.1f Stars", overallRating)
	}

	review := &model.Review{
		BookingID:         tripDetails.BookingID,
		BookingRef:        req.BookingRef,
		UserID:            userID,
		BusID:             tripDetails.BusID,
		OperatorID:        tripDetails.OperatorID,
		ScheduleID:        tripDetails.ScheduleID,
		Rating:            overallRating,
		CleanlinessRating: req.CleanlinessRating,
		PunctualityRating: req.PunctualityRating,
		ComfortRating:     req.ComfortRating,
		StaffRating:       req.StaffRating,
		Headline:          headline,
		Comment:           strings.TrimSpace(req.Comment),
	}

	if err := s.repo.CreateReview(ctx, review); err != nil {
		return nil, fmt.Errorf("failed to save review: %w", err)
	}

	return s.repo.GetReviewByBookingRef(ctx, req.BookingRef)
}

// GetBookingReview fetches the passenger review for a booking if one exists
func (s *ReviewService) GetBookingReview(ctx context.Context, bookingRef string) (*model.Review, error) {
	return s.repo.GetReviewByBookingRef(ctx, bookingRef)
}

// GetBusReviews retrieves reviews and aggregate rating breakdown for a bus
func (s *ReviewService) GetBusReviews(ctx context.Context, busID string, page, limit int) (map[string]any, error) {
	if limit <= 0 {
		limit = 10
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	reviews, err := s.repo.GetReviewsByBus(ctx, busID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve bus reviews: %w", err)
	}

	summary, err := s.repo.GetBusRatingSummary(ctx, busID)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate ratings summary: %w", err)
	}

	return map[string]any{
		"summary": summary,
		"reviews": reviews,
		"page":    page,
		"limit":   limit,
	}, nil
}

// GetOperatorReviews retrieves all reviews submitted for an operator's fleet
func (s *ReviewService) GetOperatorReviews(ctx context.Context, operatorID string, page, limit int) (map[string]any, error) {
	if limit <= 0 {
		limit = 25
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	reviews, err := s.repo.GetReviewsByOperator(ctx, operatorID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve operator reviews: %w", err)
	}

	return map[string]any{
		"reviews": reviews,
		"count":   len(reviews),
		"page":    page,
		"limit":   limit,
	}, nil
}

// ReplyToReview allows the operator to post an official response to feedback
func (s *ReviewService) ReplyToReview(ctx context.Context, reviewID string, operatorID string, reply string) error {
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return errors.New("reply message cannot be empty")
	}
	return s.repo.ReplyToReview(ctx, reviewID, operatorID, reply)
}
