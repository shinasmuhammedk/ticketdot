package model

type DashboardStats struct {
	TotalBuses     int     `db:"total_buses" json:"total_buses"`
	TotalRoutes    int     `db:"total_routes" json:"total_routes"`
	TotalSchedules int     `db:"total_schedules" json:"total_schedules"`
	TotalBookings  int     `db:"total_bookings" json:"total_bookings"`
	TotalRevenue   float64 `db:"total_revenue" json:"total_revenue"`
}
