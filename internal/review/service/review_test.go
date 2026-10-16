package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"ticketdot/internal/review/model"
)

type mockReviewRepo struct {
	reviews []*model.Review
}

func (m *mockReviewRepo) CreateReview(ctx context.Context, review *model.Review) error {
	review.ID = "rev-123"
	review.CreatedAt = time.Now()
	review.UpdatedAt = time.Now()
	m.reviews = append(m.reviews, review)
	return nil
}

func (m *mockReviewRepo) GetReviewByBookingRef(ctx context.Context, bookingRef string) (*model.Review, error) {
	for _, r := range m.reviews {
		if r.BookingRef == bookingRef {
			return r, nil
		}
	}
	return nil, nil
}

func (m *mockReviewRepo) GetReviewsByBus(ctx context.Context, busID string, limit, offset int) ([]model.Review, error) {
	var result []model.Review
	for _, r := range m.reviews {
		if r.BusID == busID {
			result = append(result, *r)
		}
	}
	return result, nil
}

func (m *mockReviewRepo) GetReviewsByOperator(ctx context.Context, operatorID string, limit, offset int) ([]model.Review, error) {
	var result []model.Review
	for _, r := range m.reviews {
		if r.OperatorID == operatorID {
			result = append(result, *r)
		}
	}
	return result, nil
}

func (m *mockReviewRepo) GetBusRatingSummary(ctx context.Context, busID string) (*model.BusRatingSummary, error) {
	return &model.BusRatingSummary{
		BusID:         busID,
		AverageRating: 4.8,
		TotalReviews:  len(m.reviews),
	}, nil
}

func (m *mockReviewRepo) GetBookingTripDetails(ctx context.Context, bookingRef string) (*model.BookingTripDetails, error) {
	if bookingRef == "VALID-REF" {
		return &model.BookingTripDetails{
			BookingID:  "b-1",
			UserID:     "user-1",
			BusID:      "bus-1",
			OperatorID: "op-1",
			ScheduleID: "sched-1",
			Status:     "confirmed",
		}, nil
	}
	if bookingRef == "CANCELLED-REF" {
		return &model.BookingTripDetails{
			BookingID:  "b-2",
			UserID:     "user-1",
			BusID:      "bus-1",
			OperatorID: "op-1",
			ScheduleID: "sched-1",
			Status:     "cancelled",
		}, nil
	}
	return nil, nil
}

func (m *mockReviewRepo) ReplyToReview(ctx context.Context, reviewID string, operatorID string, reply string) error {
	for _, r := range m.reviews {
		if r.ID == reviewID {
			r.OperatorReply = &reply
			now := time.Now()
			r.OperatorRepliedAt = &now
			return nil
		}
	}
	return errors.New("review not found")
}

func TestSubmitReview(t *testing.T) {
	repo := &mockReviewRepo{}
	svc := NewReviewService(repo)
	ctx := context.Background()

	// 1. Missing booking ref
	_, err := svc.SubmitReview(ctx, "user-1", model.SubmitReviewRequest{})
	if err == nil {
		t.Fatal("expected error for empty booking ref")
	}

	// 2. Rating out of range
	_, err = svc.SubmitReview(ctx, "user-1", model.SubmitReviewRequest{
		BookingRef:        "VALID-REF",
		CleanlinessRating: 6,
	})
	if err == nil {
		t.Fatal("expected error for rating > 5")
	}

	// 3. User mismatch
	_, err = svc.SubmitReview(ctx, "wrong-user", model.SubmitReviewRequest{
		BookingRef:        "VALID-REF",
		CleanlinessRating: 5,
		PunctualityRating: 5,
		ComfortRating:     4,
		StaffRating:       5,
	})
	if err == nil {
		t.Fatal("expected error for user mismatch")
	}

	// 4. Cancelled booking
	_, err = svc.SubmitReview(ctx, "user-1", model.SubmitReviewRequest{
		BookingRef:        "CANCELLED-REF",
		CleanlinessRating: 5,
		PunctualityRating: 5,
		ComfortRating:     4,
		StaffRating:       5,
	})
	if err == nil {
		t.Fatal("expected error for cancelled booking")
	}

	// 5. Successful review submission
	rev, err := svc.SubmitReview(ctx, "user-1", model.SubmitReviewRequest{
		BookingRef:        "VALID-REF",
		CleanlinessRating: 5,
		PunctualityRating: 5,
		ComfortRating:     4,
		StaffRating:       5,
		Headline:          "Great trip",
		Comment:           "Very punctual and neat",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedRating := 4.8 // (5+5+4+5)/4.0 = 4.75 -> rounded to 4.8
	if rev.Rating != expectedRating {
		t.Fatalf("expected rating %.1f, got %.1f", expectedRating, rev.Rating)
	}

	// 6. Duplicate review prevention
	_, err = svc.SubmitReview(ctx, "user-1", model.SubmitReviewRequest{
		BookingRef:        "VALID-REF",
		CleanlinessRating: 5,
		PunctualityRating: 5,
		ComfortRating:     5,
		StaffRating:       5,
	})
	if err == nil {
		t.Fatal("expected error for duplicate review submission")
	}

	// 7. Operator reply
	err = svc.ReplyToReview(ctx, "rev-123", "op-1", "Thanks for your feedback!")
	if err != nil {
		t.Fatalf("unexpected error in reply: %v", err)
	}
	if *rev.OperatorReply != "Thanks for your feedback!" {
		t.Fatalf("reply mismatch, got: %s", *rev.OperatorReply)
	}
}
