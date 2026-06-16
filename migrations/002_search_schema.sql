-- ===================================================
-- Search Bus Schema
-- ===================================================

-- Cities
CREATE TABLE IF NOT EXISTS cities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL UNIQUE,
    state VARCHAR(100),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Operators
CREATE TABLE IF NOT EXISTS operators (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(200) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Routes
CREATE TABLE IF NOT EXISTS routes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    origin_id UUID NOT NULL REFERENCES cities(id),
    destination_id UUID NOT NULL REFERENCES cities(id),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Buses
CREATE TABLE IF NOT EXISTS buses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    operator_id UUID NOT NULL REFERENCES operators(id),
    bus_type VARCHAR(50) NOT NULL,
    total_seats INT NOT NULL,
    registration_no VARCHAR(50) UNIQUE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Schedules
CREATE TABLE IF NOT EXISTS schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    route_id UUID NOT NULL REFERENCES routes(id),
    bus_id UUID NOT NULL REFERENCES buses(id),
    operator_id UUID NOT NULL REFERENCES operators(id),

    departure_time TIMESTAMPTZ NOT NULL,
    arrival_time TIMESTAMPTZ NOT NULL,

    available_seats INT NOT NULL,
    base_fare NUMERIC(10,2) NOT NULL,

    status VARCHAR(20) NOT NULL DEFAULT 'scheduled',

    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- ===================================================
-- Seed Data
-- ===================================================

-- Cities
INSERT INTO cities (name, state)
VALUES
    ('Mumbai', 'Maharashtra'),
    ('Pune', 'Maharashtra')
ON CONFLICT (name) DO NOTHING;

-- Operator
INSERT INTO operators (name)
VALUES ('Shivneri Travels')
ON CONFLICT DO NOTHING;