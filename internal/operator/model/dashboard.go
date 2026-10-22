package model

type DashboardStats struct {
	TotalBuses        int   `db:"total_buses" json:"total_buses"`
	TotalRoutes       int   `db:"total_routes" json:"total_routes"`
	TotalSchedules    int   `db:"total_schedules" json:"total_schedules"`
	TotalBookings     int   `db:"total_bookings" json:"total_bookings"`
	TotalRevenue      int64 `db:"total_revenue" json:"total_revenue"`       // Revenue in paise
	TotalRevenuePaise int64 `db:"-" json:"total_revenue_paise"`             // Explicit paise field
}

type DailyRevenuePoint struct {
	Date         string `db:"date" json:"date"`
	Revenue      int64  `db:"revenue" json:"revenue"`             // Revenue in paise
	RevenuePaise int64  `db:"-" json:"revenue_paise"`             // Explicit paise field
	TicketsSold  int    `db:"tickets_sold" json:"tickets_sold"`
}

type RouteOccupancyStat struct {
	RouteID           string  `db:"route_id" json:"route_id"`
	Origin            string  `db:"origin" json:"origin"`
	Destination       string  `db:"destination" json:"destination"`
	Corridor          string  `json:"corridor"`
	TotalSchedules    int     `db:"total_schedules" json:"total_schedules"`
	TotalSeatsOffered int     `db:"total_seats_offered" json:"total_seats_offered"`
	TotalSeatsBooked  int     `db:"total_seats_booked" json:"total_seats_booked"`
	OccupancyRate     float64 `json:"occupancy_rate"`
	TotalRevenue      int64   `db:"total_revenue" json:"total_revenue"`       // Revenue in paise
	TotalRevenuePaise int64   `db:"-" json:"total_revenue_paise"`             // Explicit paise field
}

type BusUtilizationStat struct {
	BusID             string  `db:"bus_id" json:"bus_id"`
	RegistrationNo    string  `db:"registration_no" json:"registration_no"`
	BusType           string  `db:"bus_type" json:"bus_type"`
	TotalSeats        int     `db:"total_seats" json:"total_seats"`
	TotalTrips        int     `db:"total_trips" json:"total_trips"`
	SeatsBooked       int     `db:"seats_booked" json:"seats_booked"`
	OccupancyRate     float64 `json:"occupancy_rate"`
	TotalRevenue      int64   `db:"total_revenue" json:"total_revenue"`       // Revenue in paise
	TotalRevenuePaise int64   `db:"-" json:"total_revenue_paise"`             // Explicit paise field
}

type AnalyticsOverview struct {
	Period              string               `json:"period"`
	TotalRevenue        int64                `json:"total_revenue"`             // Revenue in paise
	TotalRevenuePaise   int64                `json:"total_revenue_paise"`       // Explicit paise field
	TotalTickets        int                  `json:"total_tickets"`
	AvgTicketPrice      int64                `json:"avg_ticket_price"`          // Average in paise
	AvgTicketPricePaise int64                `json:"avg_ticket_price_paise"`    // Explicit paise field
	TopRoute            string               `json:"top_route"`
	RevenueTimeline     []DailyRevenuePoint  `json:"revenue_timeline"`
	RouteOccupancy      []RouteOccupancyStat `json:"route_occupancy"`
	FleetUtilization    []BusUtilizationStat `json:"fleet_utilization"`
}
