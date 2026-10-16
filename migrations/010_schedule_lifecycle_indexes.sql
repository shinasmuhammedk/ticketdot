-- ===================================================
-- Schedule Lifecycle Performance Indexes
-- ===================================================

-- Index for finding scheduled trips that have departed
CREATE INDEX IF NOT EXISTS idx_schedules_lifecycle_departure
ON schedules (status, departure_time)
WHERE status = 'scheduled';

-- Index for finding departed/active trips that have completed
CREATE INDEX IF NOT EXISTS idx_schedules_lifecycle_arrival
ON schedules (status, arrival_time)
WHERE status IN ('scheduled', 'departed');
