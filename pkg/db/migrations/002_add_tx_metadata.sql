-- Migration 002: add transaction metadata field
-- Adds the metadata column to store transaction execution metadata from Tx.Metadata field.
-- This field was introduced in fabric-x-committer v1.0.3 and fabric-x-common v0.2.6.

ALTER TABLE transactions ADD COLUMN IF NOT EXISTS metadata BYTEA;

COMMENT ON COLUMN transactions.metadata IS 'Transaction execution metadata (Tx.Metadata field). Contains additional execution information that does not affect world state. Introduced in committer v1.0.3.';

-- Made with Bob
