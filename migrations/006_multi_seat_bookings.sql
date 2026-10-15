-- ===========================================
-- Multi-seat booking support: allow shared booking_ref across multiple seats
-- ===========================================

-- Drop unique constraint on booking_ref so multiple seats in the same booking can share the same booking_ref
ALTER TABLE bookings DROP CONSTRAINT IF EXISTS bookings_booking_ref_key;

-- Add index on booking_ref for efficient lookups
CREATE INDEX IF NOT EXISTS idx_bookings_booking_ref ON bookings(booking_ref);
