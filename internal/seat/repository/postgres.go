package repository

import (
	"github.com/jmoiron/sqlx"

	"ticketdot/internal/seat/model"
)

type PostgresRepository struct {
	db *sqlx.DB
}

func NewPostgresRepository(db *sqlx.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

var _ SeatRepository = (*PostgresRepository)(nil)

func (r *PostgresRepository) GetSeatsByScheduleID(scheduleID string) ([]model.Seat, error) {
	query := `
		SELECT
			sl.seat_number,
			sl.seat_type,
			sl.deck_type,
			sl.row_num,
			sl.col_num,
			sl.is_window,
			si.status
		FROM schedules s
		JOIN seat_layouts sl ON sl.bus_id = s.bus_id
		JOIN seat_inventory si 
			ON si.schedule_id = s.id
			AND si.seat_number = sl.seat_number
		WHERE s.id = $1
		ORDER BY sl.row_num, sl.col_num
	`

	seats := []model.Seat{}

	err := r.db.Select(&seats, query, scheduleID)
	if err != nil {
		return nil, err
	}

	return seats, nil
}
