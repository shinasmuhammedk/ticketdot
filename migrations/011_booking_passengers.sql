-- Migration 011: Add passenger details, boarding/dropping stops, and contact info to bookings

ALTER TABLE bookings
    ADD COLUMN IF NOT EXISTS passenger_name VARCHAR(100),
    ADD COLUMN IF NOT EXISTS passenger_age INT,
    ADD COLUMN IF NOT EXISTS passenger_gender VARCHAR(20),
    ADD COLUMN IF NOT EXISTS boarding_point VARCHAR(255),
    ADD COLUMN IF NOT EXISTS dropping_point VARCHAR(255),
    ADD COLUMN IF NOT EXISTS contact_phone VARCHAR(30),
    ADD COLUMN IF NOT EXISTS contact_email VARCHAR(255);

CREATE INDEX IF NOT EXISTS idx_bookings_schedule_passenger ON bookings(schedule_id, passenger_name);
CREATE INDEX IF NOT EXISTS idx_bookings_contact_phone ON bookings(contact_phone);
