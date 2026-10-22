-- Migration 016: Add family_id and revoked_at to refresh_tokens, remove redundant index

ALTER TABLE refresh_tokens ADD COLUMN IF NOT EXISTS family_id UUID NOT NULL DEFAULT gen_random_uuid();
ALTER TABLE refresh_tokens ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ;

-- Backfill revoked_at for any previously revoked tokens
UPDATE refresh_tokens SET revoked_at = NOW() WHERE revoked = TRUE AND revoked_at IS NULL;

-- Remove redundant index (UNIQUE constraint on token_hash already creates an index)
DROP INDEX IF EXISTS idx_refresh_tokens_hash;

-- Index for token family revocation and lookups
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_family ON refresh_tokens(family_id);
