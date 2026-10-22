-- Migration 018: Payment orders expiry tracking and concurrency index
--
-- Adds expires_at column to payments to track payment window expiration.
-- Adds partial unique index on (schedule_id, seats) where status = 'pending'
-- to guarantee that concurrent requests cannot create multiple pending orders
-- for the same seats on a schedule.
--
-- Rollback Instructions:
--   DROP INDEX IF EXISTS idx_payments_pending_schedule_seats;
--   ALTER TABLE payments DROP COLUMN IF EXISTS expires_at;

ALTER TABLE payments
  ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;

CREATE UNIQUE INDEX IF NOT EXISTS idx_payments_pending_schedule_seats
  ON payments (schedule_id, seats)
  WHERE status = 'pending';
