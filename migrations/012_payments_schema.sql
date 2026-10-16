-- Migration 012: Payment Gateway, Webhooks, and Refund Tracking Schema

CREATE TABLE IF NOT EXISTS payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id VARCHAR(100) UNIQUE NOT NULL,
    payment_id VARCHAR(100),
    booking_ref VARCHAR(30),
    user_id UUID NOT NULL REFERENCES users(id),
    schedule_id UUID NOT NULL REFERENCES schedules(id),
    seats TEXT[] NOT NULL DEFAULT '{}',
    amount NUMERIC(10,2) NOT NULL,
    currency VARCHAR(10) NOT NULL DEFAULT 'INR',
    status VARCHAR(30) NOT NULL DEFAULT 'pending', -- 'pending', 'captured', 'failed', 'refunded'
    payment_method VARCHAR(50),                   -- 'upi', 'card', 'netbanking', 'wallet'
    gateway VARCHAR(50) NOT NULL DEFAULT 'razorpay',
    gateway_signature TEXT,
    refund_id VARCHAR(100),
    refund_amount NUMERIC(10,2) DEFAULT 0,
    refund_status VARCHAR(30),                    -- 'processed', 'pending', 'failed'
    refunded_at TIMESTAMPTZ,
    error_code VARCHAR(100),
    error_description TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Performance and lookup indexes
CREATE INDEX IF NOT EXISTS idx_payments_order_id ON payments(order_id);
CREATE INDEX IF NOT EXISTS idx_payments_payment_id ON payments(payment_id);
CREATE INDEX IF NOT EXISTS idx_payments_booking_ref ON payments(booking_ref);
CREATE INDEX IF NOT EXISTS idx_payments_user_id ON payments(user_id);
CREATE INDEX IF NOT EXISTS idx_payments_status ON payments(status);
CREATE INDEX IF NOT EXISTS idx_payments_schedule_id ON payments(schedule_id);
