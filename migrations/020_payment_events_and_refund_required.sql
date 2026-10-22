-- Migration 020: Payment events log, refund_required status, and email dispatch tracking
--
-- 1. Updates chk_payments_status constraint to include 'refund_required'.
-- 2. Creates payment_events table for webhook idempotency and event audit logging.
-- 3. Adds ticket_email_sent_at to payments table to ensure exactly-once ticket email dispatch.
-- 4. Creates view_refund_required_payments view for operator/admin/test introspection.
--
-- Rollback Instructions:
--   DROP VIEW IF EXISTS view_refund_required_payments;
--   ALTER TABLE payments DROP COLUMN IF EXISTS ticket_email_sent_at;
--   DROP TABLE IF EXISTS payment_events;
--   ALTER TABLE payments DROP CONSTRAINT IF EXISTS chk_payments_status;
--   ALTER TABLE payments ADD CONSTRAINT chk_payments_status CHECK (status IN ('pending', 'captured', 'failed', 'refunded', 'expired'));

-- 1. Update status constraint to include 'refund_required'
ALTER TABLE payments
  DROP CONSTRAINT IF EXISTS chk_payments_status;

ALTER TABLE payments
  ADD CONSTRAINT chk_payments_status
  CHECK (status IN ('pending', 'captured', 'failed', 'refunded', 'expired', 'refund_required'));

-- 2. Create payment_events table for webhook event idempotency
CREATE TABLE IF NOT EXISTS payment_events (
  id BIGSERIAL PRIMARY KEY,
  event_id VARCHAR(100) NOT NULL UNIQUE,
  event_type VARCHAR(100) NOT NULL,
  order_id VARCHAR(100),
  payment_id VARCHAR(100),
  status VARCHAR(50) NOT NULL CHECK (status IN ('processed', 'ignored', 'failed')),
  error_code VARCHAR(100),
  received_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  processed_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_payment_events_order_id
  ON payment_events (order_id)
  WHERE order_id IS NOT NULL;

-- 3. Add ticket_email_sent_at column to payments to guarantee once-only email sending
ALTER TABLE payments
  ADD COLUMN IF NOT EXISTS ticket_email_sent_at TIMESTAMP WITH TIME ZONE;

-- 4. View for dev-visible listing of refund_required orders
CREATE OR REPLACE VIEW view_refund_required_payments AS
SELECT
  id,
  order_id,
  payment_id,
  gateway_payment_id,
  user_id,
  schedule_id,
  seats,
  amount,
  currency,
  status,
  last_error_code,
  created_at,
  updated_at
FROM payments
WHERE status = 'refund_required';
