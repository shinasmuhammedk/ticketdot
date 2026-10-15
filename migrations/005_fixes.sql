-- ===========================================
-- Booking Flow Fixes & Schema Enhancements
-- ===========================================

-- 1. Add lock tracking columns to seat_inventory
ALTER TABLE seat_inventory ADD COLUMN IF NOT EXISTS locked_by UUID;
ALTER TABLE seat_inventory ADD COLUMN IF NOT EXISTS locked_at TIMESTAMPTZ;
ALTER TABLE seat_inventory ADD COLUMN IF NOT EXISTS lock_expires TIMESTAMPTZ;

-- 2. Add operator_id to routes
ALTER TABLE routes ADD COLUMN IF NOT EXISTS operator_id UUID REFERENCES operators(id);

-- 3. Partial unique index on confirmed bookings to prevent double-booking a seat per schedule
CREATE UNIQUE INDEX IF NOT EXISTS idx_bookings_schedule_seat_confirmed
ON bookings (schedule_id, seat_number)
WHERE status = 'confirmed';

-- 4. Helpful index for expired lock cleaner and queries
CREATE INDEX IF NOT EXISTS idx_seat_inventory_lock_expires
ON seat_inventory (status, lock_expires)
WHERE status = 'locked';
