-- ===========================================
-- Seat Layouts (Physical Bus Layout)
-- ===========================================

CREATE TABLE IF NOT EXISTS seat_layouts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    bus_id UUID NOT NULL REFERENCES buses(id) ON DELETE CASCADE,

    seat_number VARCHAR(10) NOT NULL,
    seat_type VARCHAR(20) NOT NULL DEFAULT 'seater',
    deck_type VARCHAR(20) NOT NULL DEFAULT 'lower',

    row_num INT NOT NULL,
    col_num INT NOT NULL,

    is_window BOOLEAN NOT NULL DEFAULT FALSE,

    UNIQUE(bus_id, seat_number)
);

-- ===========================================
-- Seat Inventory (Per Schedule)
-- ===========================================

CREATE TABLE IF NOT EXISTS seat_inventory (
    schedule_id UUID NOT NULL REFERENCES schedules(id) ON DELETE CASCADE,

    seat_number VARCHAR(10) NOT NULL,

    status VARCHAR(20) NOT NULL DEFAULT 'available',

    PRIMARY KEY (schedule_id, seat_number)
);