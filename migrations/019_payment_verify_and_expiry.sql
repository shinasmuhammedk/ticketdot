-- Migration 019: Payment verification hardening and expiry tracking
--
-- 1. Adds 'expired' status and formalizes check constraint on payments.status.
-- 2. Adds gateway_payment_id column with partial unique index to prevent duplicate captures across orders.
-- 3. Adds last_error_code column for diagnostics on verification failure without marking pending orders as permanently failed.
-- 4. Adds index on payments(expires_at) WHERE status = 'pending' for high-performance 30s background worker sweeps.
--
-- Rollback Instructions:
--   DROP INDEX IF EXISTS idx_payments_pending_expiry;
--   DROP INDEX IF EXISTS idx_payments_gateway_payment_id;
--   ALTER TABLE payments DROP CONSTRAINT IF EXISTS chk_payments_status;
--   ALTER TABLE payments DROP COLUMN IF EXISTS last_error_code;
--   ALTER TABLE payments DROP COLUMN IF EXISTS gateway_payment_id;

-- 1. Status CHECK constraint allowing 'expired'
ALTER TABLE payments
  ADD CONSTRAINT chk_payments_status
  CHECK (status IN ('pending', 'captured', 'failed', 'refunded', 'expired'));

-- 2. Gateway payment ID tracking and backfill for captured payments
ALTER TABLE payments
  ADD COLUMN IF NOT EXISTS gateway_payment_id VARCHAR(100);

UPDATE payments
  SET gateway_payment_id = payment_id
  WHERE payment_id IS NOT NULL
    AND status = 'captured'
    AND gateway_payment_id IS NULL;

-- 3. Partial unique index on gateway_payment_id to prevent double captures
CREATE UNIQUE INDEX IF NOT EXISTS idx_payments_gateway_payment_id
  ON payments (gateway_payment_id)
  WHERE gateway_payment_id IS NOT NULL;

-- 4. Last error code for non-destructive failure tracking
ALTER TABLE payments
  ADD COLUMN IF NOT EXISTS last_error_code VARCHAR(100);

-- 5. Partial index for background order expiry worker sweeps
CREATE INDEX IF NOT EXISTS idx_payments_pending_expiry
  ON payments (expires_at)
  WHERE status = 'pending';
