package repository

import (
	"database/sql"

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
	operatorID string,
	req model.CreateRouteRequest,
) (*model.CreateRouteResponse, error) {

	query := `
		INSERT INTO routes (
			origin_id,
			destination_id,
			operator_id
		)
		VALUES ($1, $2, NULLIF($3, '')::uuid)
		RETURNING id, origin_id, destination_id
	`

	var route model.CreateRouteResponse

	err := r.db.QueryRowx(
		query,
		req.OriginCityID,
		req.DestinationCityID,
		operatorID,
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

	var totalSeats int
	err = tx.Get(&totalSeats, `
		SELECT COUNT(*)
		FROM seat_layouts
		WHERE bus_id = $1
	`, req.BusID)
	if err != nil {
		return nil, err
	}

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
		totalSeats,
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
			COALESCE(u.name, 'Passenger') AS passenger_name,
			COALESCE(u.email, '') AS passenger_email,
			COALESCE(u.phone, '') AS passenger_phone,
			oc.name AS origin,
			dc.name AS destination,
			COALESCE(bus.registration_no, '') AS bus_number,
			STRING_AGG(b.seat_number, ', ' ORDER BY b.seat_number) AS seat_number,
			b.status,
			COALESCE(SUM(b.amount), 0) AS amount,
			COALESCE(MIN(b.confirmed_at), NOW()) AS confirmed_at
		FROM bookings b
		JOIN schedules s ON b.schedule_id = s.id
		LEFT JOIN users u ON b.user_id = u.id
		JOIN routes r ON s.route_id = r.id
		JOIN cities oc ON r.origin_id = oc.id
		JOIN cities dc ON r.destination_id = dc.id
		JOIN buses bus ON s.bus_id = bus.id
		WHERE s.operator_id = $1
		GROUP BY
			b.booking_ref,
			b.schedule_id,
			u.name,
			u.email,
			u.phone,
			oc.name,
			dc.name,
			bus.registration_no,
			b.status
		ORDER BY confirmed_at DESC
	`

	bookings := []model.OperatorBooking{}

	err := r.db.Select(&bookings, query, operatorID)
	if err != nil {
		return nil, err
	}

	return bookings, nil
}

func (r *PostgresRepository) GetBuses(
	operatorID string,
) ([]model.CreateBusResponse, error) {

	var buses []model.CreateBusResponse

	query := `
		SELECT
			id,
			registration_no,
			bus_type,
			total_seats
		FROM buses
		WHERE operator_id = $1
		ORDER BY created_at DESC
	`

	err := r.db.Select(
		&buses,
		query,
		operatorID,
	)

	if err != nil {
		return nil, err
	}

	return buses, nil
}

func (r *PostgresRepository) GetRoutes(
	operatorID string,
) ([]model.RouteResponse, error) {

	var routes []model.RouteResponse

	query := `
	SELECT
		rt.id,
		oc.name AS origin,
		dc.name AS destination
	FROM routes rt

	JOIN cities oc
		ON rt.origin_id = oc.id

	JOIN cities dc
		ON rt.destination_id = dc.id

	WHERE (rt.operator_id = $1 OR rt.operator_id IS NULL)
	   OR EXISTS (
		SELECT 1
		FROM schedules s
		WHERE s.route_id = rt.id
		AND s.operator_id = $1
	)

	ORDER BY oc.name
	`

	err := r.db.Select(
		&routes,
		query,
		operatorID,
	)

	if err != nil {
		return nil, err
	}

	return routes, nil
}

func (r *PostgresRepository) GetSchedules(
	operatorID string,
) ([]model.ScheduleResponse, error) {

	var schedules []model.ScheduleResponse

	query := `
	SELECT
		s.id,

		oc.name AS origin,
		dc.name AS destination,

		b.registration_no AS bus_number,
		b.bus_type AS bus_type,

		s.departure_time::text AS departure_time,
		s.arrival_time::text AS arrival_time,

		s.available_seats,
		s.base_fare,
		s.status

	FROM schedules s

	JOIN routes rt
		ON s.route_id = rt.id

	JOIN cities oc
		ON rt.origin_id = oc.id

	JOIN cities dc
		ON rt.destination_id = dc.id

	JOIN buses b
		ON s.bus_id = b.id

	WHERE s.operator_id = $1

	ORDER BY s.departure_time DESC
	`

	err := r.db.Select(
		&schedules,
		query,
		operatorID,
	)

	if err != nil {
		return nil, err
	}

	return schedules, nil
}

func (r *PostgresRepository) GetDashboardStats(
	operatorID string,
) (*model.DashboardStats, error) {

	var stats model.DashboardStats

	query := `
	SELECT
	(
		SELECT COUNT(*)
		FROM buses
		WHERE operator_id = $1
	) AS total_buses,

	(
		SELECT COUNT(*)
		FROM routes rt
		JOIN schedules s
			ON rt.id = s.route_id
		WHERE s.operator_id = $1
	) AS total_routes,

	(
		SELECT COUNT(*)
		FROM schedules
		WHERE operator_id = $1
	) AS total_schedules,

	(
		SELECT COUNT(*)
		FROM bookings b
		JOIN schedules s
			ON b.schedule_id = s.id
		WHERE s.operator_id = $1
	) AS total_bookings,

	COALESCE(
	(
		SELECT SUM(b.amount)
		FROM bookings b
		JOIN schedules s
			ON b.schedule_id = s.id
		WHERE s.operator_id = $1
	),0) AS total_revenue
	`

	err := r.db.Get(
		&stats,
		query,
		operatorID,
	)

	if err != nil {
		return nil, err
	}

	return &stats, nil
}

func (r *PostgresRepository) UpdateBus(
	busID string,
	operatorID string,
	req *model.UpdateBusRequest,
) error {

	query := `
	UPDATE buses
	SET
		registration_no = $1,
		bus_type = $2,
		total_seats = $3
	WHERE id = $4
	AND operator_id = $5
	`

	result, err := r.db.Exec(
		query,
		req.RegistrationNo,
		req.BusType,
		req.TotalSeats,
		busID,
		operatorID,
	)

	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()

	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (r *PostgresRepository) DeleteBus(
	busID string,
	operatorID string,
) error {

	query := `
    DELETE FROM buses
    WHERE id = $1
    AND operator_id = $2
    `

	result, err := r.db.Exec(
		query,
		busID,
		operatorID,
	)

	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()

	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}
