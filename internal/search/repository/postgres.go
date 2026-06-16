package repository

import (
	"github.com/jmoiron/sqlx"

	"ticketdot/internal/search/model"
)

type PostgresRepository struct {
	db *sqlx.DB
}

func NewPostgresRepository(db *sqlx.DB) *PostgresRepository {
	return &PostgresRepository{
		db: db,
	}
}

var _ SearchRepository = (*PostgresRepository)(nil)

func (r *PostgresRepository) SearchBuses(from, to, date string) ([]model.BusSearchResult, error) {
	query := `
		SELECT
			s.id AS schedule_id,
			o.name AS operator_name,
			b.id AS bus_id,
			b.bus_type AS bus_type,
			s.departure_time,
			s.arrival_time,
			s.available_seats,
			s.base_fare
		FROM schedules s
		JOIN routes r ON s.route_id = r.id
		JOIN cities origin ON r.origin_id = origin.id
		JOIN cities destination ON r.destination_id = destination.id
		JOIN buses b ON s.bus_id = b.id
		JOIN operators o ON s.operator_id = o.id
		WHERE LOWER(origin.name) = LOWER($1)
		  AND LOWER(destination.name) = LOWER($2)
		  AND DATE(s.departure_time) = $3
		  AND s.status = 'scheduled'
		  AND s.available_seats > 0
		ORDER BY s.departure_time ASC
	`

	var buses []model.BusSearchResult

	err := r.db.Select(&buses, query, from, to, date)
	if err != nil {
		return nil, err
	}

	return buses, nil
}