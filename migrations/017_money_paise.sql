-- Migration 017: Convert financial amounts from decimal rupees to integer paise
--
-- This migration converts all money columns from NUMERIC(10,2) to BIGINT paise.
-- All existing rows are preserved accurately via data-preserving conversion:
--   USING ROUND(column * 100)
--
-- Columns converted to BIGINT (unit: integer paise):
--   - schedules.base_fare: Base ticket fare in paise (e.g. ₹450.00 -> 45000 paise)
--   - bookings.amount: Booking seat amount in paise (e.g. ₹450.00 -> 45000 paise)
--   - payments.amount: Payment transaction amount in paise (e.g. ₹900.00 -> 90000 paise)
--   - payments.refund_amount: Payment refund amount in paise (e.g. ₹900.00 -> 90000 paise)
--
-- Rollback Instructions:
-- To revert back to NUMERIC(10,2) decimal rupees:
--   ALTER TABLE schedules DROP CONSTRAINT IF EXISTS chk_schedules_base_fare_non_negative;
--   ALTER TABLE schedules ALTER COLUMN base_fare TYPE NUMERIC(10,2) USING (base_fare::NUMERIC / 100.0);
--
--   ALTER TABLE bookings DROP CONSTRAINT IF EXISTS chk_bookings_amount_non_negative;
--   ALTER TABLE bookings ALTER COLUMN amount TYPE NUMERIC(10,2) USING (amount::NUMERIC / 100.0);
--
--   ALTER TABLE payments DROP CONSTRAINT IF EXISTS chk_payments_amount_non_negative;
--   ALTER TABLE payments DROP CONSTRAINT IF EXISTS chk_payments_refund_amount_non_negative;
--   ALTER TABLE payments ALTER COLUMN amount TYPE NUMERIC(10,2) USING (amount::NUMERIC / 100.0);
--   ALTER TABLE payments ALTER COLUMN refund_amount TYPE NUMERIC(10,2) USING (refund_amount::NUMERIC / 100.0);

-- 1. Schedules: convert base_fare from rupees to paise
ALTER TABLE schedules
  ALTER COLUMN base_fare TYPE BIGINT USING ROUND(base_fare * 100);

ALTER TABLE schedules
  ADD CONSTRAINT chk_schedules_base_fare_non_negative CHECK (base_fare >= 0);

-- 2. Bookings: convert seat amount from rupees to paise
ALTER TABLE bookings
  ALTER COLUMN amount TYPE BIGINT USING ROUND(amount * 100);

ALTER TABLE bookings
  ADD CONSTRAINT chk_bookings_amount_non_negative CHECK (amount >= 0);

-- 3. Payments: convert amount and refund_amount from rupees to paise
ALTER TABLE payments
  ALTER COLUMN amount TYPE BIGINT USING ROUND(amount * 100);

ALTER TABLE payments
  ADD CONSTRAINT chk_payments_amount_non_negative CHECK (amount >= 0);

ALTER TABLE payments
  ALTER COLUMN refund_amount TYPE BIGINT USING ROUND(COALESCE(refund_amount, 0) * 100);

ALTER TABLE payments
  ADD CONSTRAINT chk_payments_refund_amount_non_negative CHECK (refund_amount >= 0);
