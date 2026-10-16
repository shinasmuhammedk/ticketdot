-- ========================================================
-- Migration 014: Verified Passenger Ratings & Reviews System
-- ========================================================

CREATE TABLE IF NOT EXISTS reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    booking_id UUID NOT NULL UNIQUE REFERENCES bookings(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    bus_id UUID NOT NULL REFERENCES buses(id) ON DELETE CASCADE,
    operator_id UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
    schedule_id UUID NOT NULL REFERENCES schedules(id) ON DELETE CASCADE,
    
    rating NUMERIC(2, 1) NOT NULL CHECK (rating >= 1.0 AND rating <= 5.0),
    cleanliness_rating INT NOT NULL CHECK (cleanliness_rating >= 1 AND cleanliness_rating <= 5),
    punctuality_rating INT NOT NULL CHECK (punctuality_rating >= 1 AND punctuality_rating <= 5),
    comfort_rating INT NOT NULL CHECK (comfort_rating >= 1 AND comfort_rating <= 5),
    staff_rating INT NOT NULL CHECK (staff_rating >= 1 AND staff_rating <= 5),
    
    headline VARCHAR(150),
    comment TEXT,
    
    operator_reply TEXT,
    operator_replied_at TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_reviews_bus_id ON reviews(bus_id);
CREATE INDEX IF NOT EXISTS idx_reviews_operator_id ON reviews(operator_id);
CREATE INDEX IF NOT EXISTS idx_reviews_user_id ON reviews(user_id);
CREATE INDEX IF NOT EXISTS idx_reviews_schedule_id ON reviews(schedule_id);

-- Optional seed for first available confirmed booking
INSERT INTO reviews (booking_id, user_id, bus_id, operator_id, schedule_id, rating, cleanliness_rating, punctuality_rating, comfort_rating, staff_rating, headline, comment, operator_reply, operator_replied_at)
SELECT 
    b.id,
    b.user_id,
    s.bus_id,
    s.operator_id,
    s.id,
    4.8,
    5,
    5,
    4,
    5,
    'Exceptional journey experience! Smooth ride and very clean.',
    'The bus arrived 10 minutes prior to departure time at the boarding station. Air conditioning was pleasant, charging ports were working, and the crew was helpful. Highly recommended!',
    'Thank you so much for traveling with us! We look forward to serving you again on your next trip.',
    NOW()
FROM bookings b
JOIN schedules s ON b.schedule_id = s.id
LIMIT 1
ON CONFLICT (booking_id) DO NOTHING;
