package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"

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

	// If route already exists for this operator or globally, return it idempotently
	var existing model.CreateRouteResponse
	err := r.db.QueryRowx(`
		SELECT id, origin_id, destination_id
		FROM routes
		WHERE origin_id = $1 AND destination_id = $2
		  AND (operator_id = NULLIF($3, '')::uuid OR operator_id IS NULL)
		LIMIT 1
	`, req.OriginCityID, req.DestinationCityID, operatorID).Scan(
		&existing.ID,
		&existing.OriginCityID,
		&existing.DestinationCityID,
	)
	if err == nil {
		return &existing, nil
	}

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

	err = r.db.QueryRowx(
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
	farePaise int64,
) (*model.CreateScheduleResponse, error) {

	tx, err := r.db.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 1. Verify bus exists and belongs to operator
	var busTotalSeats int
	err = tx.Get(&busTotalSeats, `
		SELECT total_seats
		FROM buses
		WHERE id = $1 AND operator_id = $2
	`, req.BusID, operatorID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("assigned bus not found in your fleet")
		}
		return nil, err
	}

	// 2. Verify route exists
	var routeExists bool
	err = tx.Get(&routeExists, `
		SELECT EXISTS(
			SELECT 1 FROM routes
			WHERE id = $1 AND (operator_id = NULLIF($2, '')::uuid OR operator_id IS NULL)
		)
	`, req.RouteID, operatorID)
	if err != nil || !routeExists {
		return nil, errors.New("assigned route not found in operational network")
	}

	// 3. Count physical seats configured for this bus
	var totalSeats int
	err = tx.Get(&totalSeats, `
		SELECT COUNT(*)
		FROM seat_layouts
		WHERE bus_id = $1
	`, req.BusID)
	if err != nil {
		return nil, err
	}
	if totalSeats == 0 {
		return nil, errors.New("assigned bus has no seat layouts initialized")
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
		farePaise,
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

	schedule.BaseFarePaise = schedule.BaseFare

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
			COALESCE(NULLIF(MAX(b.passenger_name), ''), u.name, 'Passenger') AS passenger_name,
			COALESCE(NULLIF(MAX(b.contact_email), ''), u.email, '') AS passenger_email,
			COALESCE(NULLIF(MAX(b.contact_phone), ''), u.phone, '') AS passenger_phone,
			COALESCE(MAX(b.boarding_point), '') AS boarding_point,
			COALESCE(MAX(b.dropping_point), '') AS dropping_point,
			COALESCE(
				STRING_AGG(
					DISTINCT CONCAT(
						b.seat_number, ': ',
						COALESCE(NULLIF(b.passenger_name, ''), u.name, 'Passenger'),
						CASE 
							WHEN b.passenger_age IS NOT NULL AND b.passenger_gender IS NOT NULL AND b.passenger_gender != '' THEN CONCAT(' (', b.passenger_age, '/', b.passenger_gender, ')')
							WHEN b.passenger_age IS NOT NULL THEN CONCAT(' (', b.passenger_age, ' yrs)')
							WHEN b.passenger_gender IS NOT NULL AND b.passenger_gender != '' THEN CONCAT(' (', b.passenger_gender, ')')
							ELSE ''
						END
					),
					'; '
					ORDER BY CONCAT(
						b.seat_number, ': ',
						COALESCE(NULLIF(b.passenger_name, ''), u.name, 'Passenger'),
						CASE 
							WHEN b.passenger_age IS NOT NULL AND b.passenger_gender IS NOT NULL AND b.passenger_gender != '' THEN CONCAT(' (', b.passenger_age, '/', b.passenger_gender, ')')
							WHEN b.passenger_age IS NOT NULL THEN CONCAT(' (', b.passenger_age, ' yrs)')
							WHEN b.passenger_gender IS NOT NULL AND b.passenger_gender != '' THEN CONCAT(' (', b.passenger_gender, ')')
							ELSE ''
						END
					)
				),
				''
			) AS passenger_details,
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

	for i := range bookings {
		bookings[i].AmountPaise = bookings[i].Amount
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

	for i := range schedules {
		schedules[i].BaseFarePaise = schedules[i].BaseFare
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

	stats.TotalRevenuePaise = stats.TotalRevenue

	return &stats, nil
}

func (r *PostgresRepository) UpdateBus(
	busID string,
	operatorID string,
	req *model.UpdateBusRequest,
) error {
	tx, err := r.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Check if bus exists and get current values
	var current struct {
		TotalSeats int    `db:"total_seats"`
		RegNo      string `db:"registration_no"`
	}
	err = tx.Get(&current, `
		SELECT total_seats, registration_no
		FROM buses
		WHERE id = $1 AND operator_id = $2
	`, busID, operatorID)
	if err != nil {
		return err
	}

	// 2. Check if registration number is already used by another bus
	var duplicateCount int
	err = tx.Get(&duplicateCount, `
		SELECT COUNT(*)
		FROM buses
		WHERE registration_no = $1 AND id != $2
	`, req.RegistrationNo, busID)
	if err == nil && duplicateCount > 0 {
		return errors.New("registration number already exists for another vehicle")
	}

	// 3. If total seats changed, check if schedules exist for this bus
	if req.TotalSeats != current.TotalSeats {
		var scheduleCount int
		err = tx.Get(&scheduleCount, `
			SELECT COUNT(*)
			FROM schedules
			WHERE bus_id = $1
		`, busID)
		if err != nil {
			return err
		}
		if scheduleCount > 0 {
			return errors.New("cannot change seating capacity: vehicle has existing trip schedules")
		}

		// Re-initialize physical seat layout for the updated capacity
		_, err = tx.Exec(`DELETE FROM seat_layouts WHERE bus_id = $1`, busID)
		if err != nil {
			return err
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
		`, busID, req.TotalSeats)
		if err != nil {
			return err
		}
	}

	result, err := tx.Exec(`
		UPDATE buses
		SET
			registration_no = $1,
			bus_type = $2,
			total_seats = $3
		WHERE id = $4 AND operator_id = $5
	`, req.RegistrationNo, req.BusType, req.TotalSeats, busID, operatorID)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}

	return tx.Commit()
}

func (r *PostgresRepository) DeleteBus(
	busID string,
	operatorID string,
) error {
	// 1. Verify bus exists
	var exists bool
	err := r.db.Get(&exists, `
		SELECT EXISTS(SELECT 1 FROM buses WHERE id = $1 AND operator_id = $2)
	`, busID, operatorID)
	if err != nil {
		return err
	}
	if !exists {
		return sql.ErrNoRows
	}

	// 2. Check if assigned to any trip schedules
	var scheduleCount int
	err = r.db.Get(&scheduleCount, `
		SELECT COUNT(*) FROM schedules WHERE bus_id = $1
	`, busID)
	if err != nil {
		return err
	}
	if scheduleCount > 0 {
		return errors.New("cannot delete bus: vehicle is assigned to active trip schedules")
	}

	// 3. Delete bus (seat_layouts will cascade delete)
	result, err := r.db.Exec(`
		DELETE FROM buses
		WHERE id = $1 AND operator_id = $2
	`, busID, operatorID)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (r *PostgresRepository) GetBusLayout(
	busID string,
	operatorID string,
) ([]model.BusSeatLayout, error) {
	// Verify bus belongs to operator
	var exists bool
	err := r.db.Get(&exists, `
		SELECT EXISTS(SELECT 1 FROM buses WHERE id = $1 AND operator_id = $2)
	`, busID, operatorID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, sql.ErrNoRows
	}

	var layout []model.BusSeatLayout
	query := `
		SELECT
			seat_number,
			seat_type,
			deck_type,
			row_num,
			col_num,
			is_window
		FROM seat_layouts
		WHERE bus_id = $1
		ORDER BY row_num, col_num
	`
	err = r.db.Select(&layout, query, busID)
	if err != nil {
		return nil, err
	}

	return layout, nil
}

func (r *PostgresRepository) CancelSchedule(
	scheduleID string,
	operatorID string,
) error {
	tx, err := r.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Verify schedule belongs to operator and check current status
	var currentStatus string
	err = tx.Get(&currentStatus, `
		SELECT status
		FROM schedules
		WHERE id = $1 AND operator_id = $2
		FOR UPDATE
	`, scheduleID, operatorID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		return err
	}

	if currentStatus == "cancelled" {
		return errors.New("schedule is already cancelled")
	}
	if currentStatus == "completed" {
		return errors.New("cannot cancel a completed trip schedule")
	}

	// 2. Update schedule status to cancelled
	_, err = tx.Exec(`
		UPDATE schedules
		SET status = 'cancelled'
		WHERE id = $1 AND operator_id = $2
	`, scheduleID, operatorID)
	if err != nil {
		return err
	}

	// 3. Release any locked seats for this schedule
	_, err = tx.Exec(`
		UPDATE seat_inventory
		SET
			status = 'available',
			locked_by = NULL,
			locked_at = NULL,
			lock_expires = NULL
		WHERE schedule_id = $1 AND status = 'locked'
	`, scheduleID)
	if err != nil {
		return err
	}

	// 4. Mark all confirmed bookings for this schedule as cancelled
	_, err = tx.Exec(`
		UPDATE bookings
		SET status = 'cancelled'
		WHERE schedule_id = $1 AND status = 'confirmed'
	`, scheduleID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *PostgresRepository) GetAnalytics(
	operatorID string,
	periodDays int,
) (*model.AnalyticsOverview, error) {
	if periodDays <= 0 {
		periodDays = 7
	}
	periodStr := fmt.Sprintf("%dd", periodDays)

	overview := &model.AnalyticsOverview{
		Period:           periodStr,
		RevenueTimeline:  []model.DailyRevenuePoint{},
		RouteOccupancy:   []model.RouteOccupancyStat{},
		FleetUtilization: []model.BusUtilizationStat{},
	}

	// 1. Fetch daily revenue points in the period
	timelineQuery := `
		SELECT
			TO_CHAR(b.confirmed_at, 'YYYY-MM-DD') AS date,
			COALESCE(SUM(b.amount), 0) AS revenue,
			COUNT(b.id) AS tickets_sold
		FROM bookings b
		JOIN schedules s ON b.schedule_id = s.id
		WHERE s.operator_id = $1
		  AND b.status = 'confirmed'
		  AND b.confirmed_at >= CURRENT_DATE - ($2::text || ' days')::INTERVAL
		GROUP BY TO_CHAR(b.confirmed_at, 'YYYY-MM-DD')
		ORDER BY date ASC
	`
	var points []model.DailyRevenuePoint
	err := r.db.Select(&points, timelineQuery, operatorID, periodDays)
	if err != nil {
		return nil, err
	}

	pointMap := make(map[string]model.DailyRevenuePoint)
	for _, p := range points {
		pointMap[p.Date] = p
	}

	now := time.Now().UTC()
	var totalRevenue int64
	var totalTickets int

	for i := periodDays - 1; i >= 0; i-- {
		day := now.AddDate(0, 0, -i).Format("2006-01-02")
		if existing, ok := pointMap[day]; ok {
			existing.RevenuePaise = existing.Revenue
			overview.RevenueTimeline = append(overview.RevenueTimeline, existing)
			totalRevenue += existing.Revenue
			totalTickets += existing.TicketsSold
		} else {
			overview.RevenueTimeline = append(overview.RevenueTimeline, model.DailyRevenuePoint{
				Date:         day,
				Revenue:      0,
				RevenuePaise: 0,
				TicketsSold:  0,
			})
		}
	}

	overview.TotalRevenue = totalRevenue
	overview.TotalRevenuePaise = totalRevenue
	overview.TotalTickets = totalTickets
	if totalTickets > 0 {
		overview.AvgTicketPrice = totalRevenue / int64(totalTickets)
		overview.AvgTicketPricePaise = overview.AvgTicketPrice
	}

	// 2. Route Occupancy and Performance
	routeQuery := `
		SELECT
			r.id AS route_id,
			oc.name AS origin,
			dc.name AS destination,
			COUNT(DISTINCT s.id) AS total_schedules,
			COALESCE(SUM(bus.total_seats), 0) AS total_seats_offered,
			COALESCE(COUNT(CASE WHEN b.status = 'confirmed' THEN 1 END), 0) AS total_seats_booked,
			COALESCE(SUM(CASE WHEN b.status = 'confirmed' THEN b.amount ELSE 0 END), 0) AS total_revenue
		FROM routes r
		JOIN cities oc ON r.origin_id = oc.id
		JOIN cities dc ON r.destination_id = dc.id
		JOIN schedules s ON s.route_id = r.id
		JOIN buses bus ON s.bus_id = bus.id
		LEFT JOIN bookings b ON b.schedule_id = s.id
		WHERE s.operator_id = $1
		GROUP BY r.id, oc.name, dc.name
		ORDER BY total_revenue DESC
	`
	var routeStats []model.RouteOccupancyStat
	err = r.db.Select(&routeStats, routeQuery, operatorID)
	if err == nil {
		for i := range routeStats {
			routeStats[i].TotalRevenuePaise = routeStats[i].TotalRevenue
			routeStats[i].Corridor = fmt.Sprintf("%s → %s", routeStats[i].Origin, routeStats[i].Destination)
			if routeStats[i].TotalSeatsOffered > 0 {
				rate := (float64(routeStats[i].TotalSeatsBooked) / float64(routeStats[i].TotalSeatsOffered)) * 100
				routeStats[i].OccupancyRate = math.Round(rate*10) / 10
			}
		}
		overview.RouteOccupancy = routeStats
		if len(routeStats) > 0 {
			overview.TopRoute = routeStats[0].Corridor
		}
	}

	// 3. Fleet Vehicle Utilization
	fleetQuery := `
		SELECT
			bus.id AS bus_id,
			bus.registration_no,
			bus.bus_type,
			bus.total_seats,
			COUNT(DISTINCT s.id) AS total_trips,
			COALESCE(COUNT(CASE WHEN b.status = 'confirmed' THEN 1 END), 0) AS seats_booked,
			COALESCE(SUM(CASE WHEN b.status = 'confirmed' THEN b.amount ELSE 0 END), 0) AS total_revenue
		FROM buses bus
		LEFT JOIN schedules s ON s.bus_id = bus.id AND s.operator_id = $1
		LEFT JOIN bookings b ON b.schedule_id = s.id
		WHERE bus.operator_id = $1
		GROUP BY bus.id, bus.registration_no, bus.bus_type, bus.total_seats
		ORDER BY total_revenue DESC
	`
	var fleetStats []model.BusUtilizationStat
	err = r.db.Select(&fleetStats, fleetQuery, operatorID)
	if err == nil {
		for i := range fleetStats {
			fleetStats[i].TotalRevenuePaise = fleetStats[i].TotalRevenue
			totalPossibleSeats := fleetStats[i].TotalTrips * fleetStats[i].TotalSeats
			if totalPossibleSeats > 0 {
				rate := (float64(fleetStats[i].SeatsBooked) / float64(totalPossibleSeats)) * 100
				fleetStats[i].OccupancyRate = math.Round(rate*10) / 10
			}
		}
		overview.FleetUtilization = fleetStats
	}

	return overview, nil
}
