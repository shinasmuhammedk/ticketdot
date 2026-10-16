package repository

import (
	"context"
	"database/sql"
	"errors"
	"ticketdot/internal/review/model"

	"github.com/jmoiron/sqlx"
)

type PostgresReviewRepository struct {
	db *sqlx.DB
}

func NewPostgresReviewRepository(db *sqlx.DB) *PostgresReviewRepository {
	return &PostgresReviewRepository{db: db}
}

func (r *PostgresReviewRepository) CreateReview(ctx context.Context, review *model.Review) error {
	query := `
		INSERT INTO reviews (
			booking_id, user_id, bus_id, operator_id, schedule_id,
			rating, cleanliness_rating, punctuality_rating, comfort_rating, staff_rating,
			headline, comment, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), NOW()
		)
		RETURNING id, created_at, updated_at
	`
	return r.db.QueryRowContext(
		ctx, query,
		review.BookingID, review.UserID, review.BusID, review.OperatorID, review.ScheduleID,
		review.Rating, review.CleanlinessRating, review.PunctualityRating, review.ComfortRating, review.StaffRating,
		review.Headline, review.Comment,
	).Scan(&review.ID, &review.CreatedAt, &review.UpdatedAt)
}

func (r *PostgresReviewRepository) GetReviewByBookingRef(ctx context.Context, bookingRef string) (*model.Review, error) {
	query := `
		SELECT 
			r.id, r.booking_id, r.user_id, COALESCE(u.name, 'Passenger') AS user_name,
			r.bus_id, b.bus_type, r.operator_id, o.name AS operator_name,
			r.schedule_id, bk.booking_ref, orig.name AS origin, dest.name AS destination,
			r.rating, r.cleanliness_rating, r.punctuality_rating, r.comfort_rating, r.staff_rating,
			r.headline, r.comment, r.operator_reply, r.operator_replied_at, r.created_at, r.updated_at
		FROM reviews r
		JOIN bookings bk ON r.booking_id = bk.id
		JOIN users u ON r.user_id = u.id
		JOIN buses b ON r.bus_id = b.id
		JOIN operators o ON r.operator_id = o.id
		JOIN schedules s ON r.schedule_id = s.id
		JOIN routes rt ON s.route_id = rt.id
		JOIN cities orig ON rt.origin_id = orig.id
		JOIN cities dest ON rt.destination_id = dest.id
		WHERE bk.booking_ref = $1
	`
	var rev model.Review
	if err := r.db.GetContext(ctx, &rev, query, bookingRef); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &rev, nil
}

func (r *PostgresReviewRepository) GetReviewsByBus(ctx context.Context, busID string, limit, offset int) ([]model.Review, error) {
	if limit <= 0 {
		limit = 20
	}
	query := `
		SELECT 
			r.id, r.booking_id, r.user_id, COALESCE(u.name, 'Verified Traveler') AS user_name,
			r.bus_id, b.bus_type, r.operator_id, o.name AS operator_name,
			r.schedule_id, bk.booking_ref, orig.name AS origin, dest.name AS destination,
			r.rating, r.cleanliness_rating, r.punctuality_rating, r.comfort_rating, r.staff_rating,
			r.headline, r.comment, r.operator_reply, r.operator_replied_at, r.created_at, r.updated_at
		FROM reviews r
		JOIN bookings bk ON r.booking_id = bk.id
		JOIN users u ON r.user_id = u.id
		JOIN buses b ON r.bus_id = b.id
		JOIN operators o ON r.operator_id = o.id
		JOIN schedules s ON r.schedule_id = s.id
		JOIN routes rt ON s.route_id = rt.id
		JOIN cities orig ON rt.origin_id = orig.id
		JOIN cities dest ON rt.destination_id = dest.id
		WHERE r.bus_id = $1
		ORDER BY r.created_at DESC
		LIMIT $2 OFFSET $3
	`
	var revs []model.Review
	if err := r.db.SelectContext(ctx, &revs, query, busID, limit, offset); err != nil {
		return nil, err
	}
	return revs, nil
}

func (r *PostgresReviewRepository) GetReviewsByOperator(ctx context.Context, operatorID string, limit, offset int) ([]model.Review, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
		SELECT 
			r.id, r.booking_id, r.user_id, COALESCE(u.name, 'Passenger') AS user_name,
			r.bus_id, b.bus_type, r.operator_id, o.name AS operator_name,
			r.schedule_id, bk.booking_ref, orig.name AS origin, dest.name AS destination,
			r.rating, r.cleanliness_rating, r.punctuality_rating, r.comfort_rating, r.staff_rating,
			r.headline, r.comment, r.operator_reply, r.operator_replied_at, r.created_at, r.updated_at
		FROM reviews r
		JOIN bookings bk ON r.booking_id = bk.id
		JOIN users u ON r.user_id = u.id
		JOIN buses b ON r.bus_id = b.id
		JOIN operators o ON r.operator_id = o.id
		JOIN schedules s ON r.schedule_id = s.id
		JOIN routes rt ON s.route_id = rt.id
		JOIN cities orig ON rt.origin_id = orig.id
		JOIN cities dest ON rt.destination_id = dest.id
		WHERE (r.operator_id::text = $1 OR r.operator_id IN (SELECT operator_id FROM users WHERE id::text = $1))
		ORDER BY r.created_at DESC
		LIMIT $2 OFFSET $3
	`
	var revs []model.Review
	if err := r.db.SelectContext(ctx, &revs, query, operatorID, limit, offset); err != nil {
		return nil, err
	}
	return revs, nil
}

func (r *PostgresReviewRepository) GetBusRatingSummary(ctx context.Context, busID string) (*model.BusRatingSummary, error) {
	query := `
		SELECT 
			COUNT(*) AS total_reviews,
			COALESCE(ROUND(AVG(rating), 1), 0.0) AS average_rating,
			COALESCE(ROUND(AVG(cleanliness_rating), 1), 0.0) AS cleanliness_avg,
			COALESCE(ROUND(AVG(punctuality_rating), 1), 0.0) AS punctuality_avg,
			COALESCE(ROUND(AVG(comfort_rating), 1), 0.0) AS comfort_avg,
			COALESCE(ROUND(AVG(staff_rating), 1), 0.0) AS staff_avg,
			COUNT(*) FILTER (WHERE rating >= 4.5) AS star_5,
			COUNT(*) FILTER (WHERE rating >= 3.5 AND rating < 4.5) AS star_4,
			COUNT(*) FILTER (WHERE rating >= 2.5 AND rating < 3.5) AS star_3,
			COUNT(*) FILTER (WHERE rating >= 1.5 AND rating < 2.5) AS star_2,
			COUNT(*) FILTER (WHERE rating < 1.5) AS star_1
		FROM reviews
		WHERE bus_id = $1
	`
	var row struct {
		TotalReviews   int     `db:"total_reviews"`
		AverageRating  float64 `db:"average_rating"`
		CleanlinessAvg float64 `db:"cleanliness_avg"`
		PunctualityAvg float64 `db:"punctuality_avg"`
		ComfortAvg     float64 `db:"comfort_avg"`
		StaffAvg       float64 `db:"staff_avg"`
		Star5          int     `db:"star_5"`
		Star4          int     `db:"star_4"`
		Star3          int     `db:"star_3"`
		Star2          int     `db:"star_2"`
		Star1          int     `db:"star_1"`
	}

	if err := r.db.GetContext(ctx, &row, query, busID); err != nil {
		return nil, err
	}

	return &model.BusRatingSummary{
		BusID:          busID,
		AverageRating:  row.AverageRating,
		TotalReviews:   row.TotalReviews,
		CleanlinessAvg: row.CleanlinessAvg,
		PunctualityAvg: row.PunctualityAvg,
		ComfortAvg:     row.ComfortAvg,
		StaffAvg:       row.StaffAvg,
		RatingDistribution: map[string]int{
			"5": row.Star5,
			"4": row.Star4,
			"3": row.Star3,
			"2": row.Star2,
			"1": row.Star1,
		},
	}, nil
}

func (r *PostgresReviewRepository) GetBookingTripDetails(ctx context.Context, bookingRef string) (*model.BookingTripDetails, error) {
	query := `
		SELECT 
			b.id AS booking_id,
			b.user_id,
			s.bus_id,
			s.operator_id,
			s.id AS schedule_id,
			b.status
		FROM bookings b
		JOIN schedules s ON b.schedule_id = s.id
		WHERE b.booking_ref = $1
		LIMIT 1
	`
	var d model.BookingTripDetails
	if err := r.db.GetContext(ctx, &d, query, bookingRef); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &d, nil
}

func (r *PostgresReviewRepository) ReplyToReview(ctx context.Context, reviewID string, operatorID string, reply string) error {
	query := `
		UPDATE reviews
		SET operator_reply = $1, operator_replied_at = NOW(), updated_at = NOW()
		WHERE id::text = $2 AND (operator_id::text = $3 OR operator_id IN (SELECT operator_id FROM users WHERE id::text = $3))
	`
	res, err := r.db.ExecContext(ctx, query, reply, reviewID, operatorID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("review not found or unauthorized to reply")
	}
	return nil
}
