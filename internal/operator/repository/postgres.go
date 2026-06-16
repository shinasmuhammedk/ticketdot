package repository

import (
	"github.com/jmoiron/sqlx"

	"ticketdot/internal/operator/model"
)

type PostgresRepository struct {
	db *sqlx.DB
}

func NewPostgresRepository(db *sqlx.DB) *PostgresRepository {
	return &PostgresRepository{
		db: db,
	}
}

var _ OperatorRepository = (*PostgresRepository)(nil)

func (r *PostgresRepository) CreateBus(
	operatorID string,
	req model.CreateBusRequest,
) (*model.CreateBusResponse, error) {

	tx, err := r.db.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var bus model.CreateBusResponse

	err = tx.QueryRowx(`
		INSERT INTO buses (
			operator_id,
			registration_no,
			bus_type,
			total_seats
		)
		VALUES ($1, $2, $3, $4)
		RETURNING id, registration_no, bus_type, total_seats
	`,
		operatorID,
		req.RegistrationNo,
		req.BusType,
		req.TotalSeats,
	).Scan(
		&bus.ID,
		&bus.RegistrationNo,
		&bus.BusType,
		&bus.TotalSeats,
	)

	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(`
		INSERT INTO seat_layouts (
			bus_id,
			seat_number,
			row_num,
			col_num,
			is_window
		)
		SELECT
			$1,
			CHR(64 + col) || row,
			row,
			col,
			col IN (1, 4)
		FROM generate_series(1, CEIL($2::numeric / 4)::int) AS row
		CROSS JOIN generate_series(1, 4) AS col
		WHERE ((row - 1) * 4 + col) <= $2
		ON CONFLICT DO NOTHING
	`, bus.ID, req.TotalSeats)

	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &bus, nil
}

func (r *PostgresRepository) CreateRoute(
	req model.CreateRouteRequest,
) (*model.CreateRouteResponse, error) {

	query := `
		INSERT INTO routes (
			origin_id,
			destination_id
		)
		VALUES ($1, $2)
		RETURNING id, origin_id, destination_id
	`

	var route model.CreateRouteResponse

	err := r.db.QueryRowx(
		query,
		req.OriginCityID,
		req.DestinationCityID,
	).Scan(
		&route.ID,
		&route.OriginCityID,
		&route.DestinationCityID,
	)

	if err != nil {
		return nil, err
	}

	return &route, nil
}


func (r *PostgresRepository) CreateSchedule(
	operatorID string,
	req model.CreateScheduleRequest,
) (*model.CreateScheduleResponse, error) {

	tx, err := r.db.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var schedule model.CreateScheduleResponse

	err = tx.QueryRowx(
		`
		INSERT INTO schedules (
			route_id,
			bus_id,
			operator_id,
			departure_time,
			arrival_time,
			available_seats,
			base_fare,
			status
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'scheduled')
		RETURNING
			id,
			route_id,
			bus_id,
			operator_id,
			departure_time,
			arrival_time,
			available_seats,
			base_fare,
			status
		`,
		req.RouteID,
		req.BusID,
		operatorID,
		req.DepartureTime,
		req.ArrivalTime,
		req.AvailableSeats,
		req.BaseFare,
	).Scan(
		&schedule.ID,
		&schedule.RouteID,
		&schedule.BusID,
		&schedule.OperatorID,
		&schedule.DepartureTime,
		&schedule.ArrivalTime,
		&schedule.AvailableSeats,
		&schedule.BaseFare,
		&schedule.Status,
	)

	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(
		`
		INSERT INTO seat_inventory (
			schedule_id,
			seat_number,
			status
		)
		SELECT
			$1,
			seat_number,
			'available'
		FROM seat_layouts
		WHERE bus_id = $2
		ON CONFLICT DO NOTHING
		`,
		schedule.ID,
		req.BusID,
	)

	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &schedule, nil
}

func (r *PostgresRepository) GetBookings(
	operatorID string,
) ([]model.OperatorBooking, error) {

	query := `
		SELECT
			b.booking_ref,
			b.schedule_id,
			b.seat_number,
			b.status,
			b.amount,
			b.confirmed_at
		FROM bookings b
		JOIN schedules s
			ON b.schedule_id = s.id
		WHERE s.operator_id = $1
		ORDER BY b.confirmed_at DESC
	`

	bookings := []model.OperatorBooking{}

	err := r.db.Select(&bookings, query, operatorID)
	if err != nil {
		return nil, err
	}

	return bookings, nil
}