-- ========================================================
-- Migration 013: Real-Time Bus Live GPS Tracking & Trip Status
-- ========================================================

-- 1. Extend cities table with GPS Coordinates
ALTER TABLE cities ADD COLUMN IF NOT EXISTS latitude NUMERIC(10, 6);
ALTER TABLE cities ADD COLUMN IF NOT EXISTS longitude NUMERIC(10, 6);

-- Populate accurate geocoordinates for major transit hubs
UPDATE cities SET latitude = 19.0760, longitude = 72.8777 WHERE name = 'Mumbai';
UPDATE cities SET latitude = 18.5204, longitude = 73.8567 WHERE name = 'Pune';
UPDATE cities SET latitude = 12.9716, longitude = 77.5946 WHERE name = 'Bengaluru';
UPDATE cities SET latitude = 13.0827, longitude = 80.2707 WHERE name = 'Chennai';
UPDATE cities SET latitude = 17.3850, longitude = 78.4867 WHERE name = 'Hyderabad';
UPDATE cities SET latitude = 28.6139, longitude = 77.2090 WHERE name = 'Delhi';
UPDATE cities SET latitude = 15.2993, longitude = 74.1240 WHERE name = 'Goa';
UPDATE cities SET latitude = 9.9312,  longitude = 76.2673 WHERE name = 'Kochi';
UPDATE cities SET latitude = 11.0168, longitude = 76.9558 WHERE name = 'Coimbatore';
UPDATE cities SET latitude = 23.0225, longitude = 72.5714 WHERE name = 'Ahmedabad';
UPDATE cities SET latitude = 26.9124, longitude = 75.7873 WHERE name = 'Jaipur';
UPDATE cities SET latitude = 22.5726, longitude = 88.3639 WHERE name = 'Kolkata';
UPDATE cities SET latitude = 16.5062, longitude = 80.6480 WHERE name = 'Vijayawada';
UPDATE cities SET latitude = 12.9141, longitude = 74.8560 WHERE name = 'Mangaluru';
UPDATE cities SET latitude = 9.9252,  longitude = 78.1198 WHERE name = 'Madurai';

-- Fallback for any other cities with NULL coordinates (default to central India)
UPDATE cities SET latitude = 20.5937, longitude = 78.9629 WHERE latitude IS NULL;

-- 2. Real-Time Trip Tracking Table
CREATE TABLE IF NOT EXISTS trip_trackings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    schedule_id UUID NOT NULL UNIQUE REFERENCES schedules(id) ON DELETE CASCADE,
    current_latitude NUMERIC(10, 6) NOT NULL,
    current_longitude NUMERIC(10, 6) NOT NULL,
    speed_kmh NUMERIC(5, 2) NOT NULL DEFAULT 0,
    heading_degrees NUMERIC(5, 2) NOT NULL DEFAULT 0,
    trip_status VARCHAR(30) NOT NULL DEFAULT 'scheduled', -- 'scheduled', 'boarding', 'in_transit', 'delayed', 'completed'
    delay_minutes INT NOT NULL DEFAULT 0,
    current_stop_name VARCHAR(150),
    next_stop_name VARCHAR(150),
    next_stop_eta TIMESTAMPTZ,
    progress_percent NUMERIC(5, 2) NOT NULL DEFAULT 0,
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Index for instant lookup by schedule
CREATE INDEX IF NOT EXISTS idx_trip_trackings_schedule ON trip_trackings(schedule_id);
CREATE INDEX IF NOT EXISTS idx_trip_trackings_status ON trip_trackings(trip_status);
