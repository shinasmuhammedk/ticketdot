-- ===================================================
-- Performance Indexes for High-Volume Production Queries
-- ===================================================

-- Bookings: Index by user_id and status for fast My Bookings retrieval
CREATE INDEX IF NOT EXISTS idx_bookings_user_status ON bookings(user_id, status);

-- Bookings: Index by schedule_id for manifest and operator queries
CREATE INDEX IF NOT EXISTS idx_bookings_schedule_id ON bookings(schedule_id);

-- Schedules: Composite index by route_id and departure_time for core bus search
CREATE INDEX IF NOT EXISTS idx_schedules_route_departure ON schedules(route_id, departure_time);

-- Schedules: Index by operator_id for operator dashboard and schedules management
CREATE INDEX IF NOT EXISTS idx_schedules_operator_id ON schedules(operator_id);

-- Routes: Index by origin_id and destination_id for instant route resolution
CREATE INDEX IF NOT EXISTS idx_routes_origin_destination ON routes(origin_id, destination_id);

-- Seat Inventory: Index by schedule_id and status for fast seat layout queries
CREATE INDEX IF NOT EXISTS idx_seat_inventory_schedule_status ON seat_inventory(schedule_id, status);
