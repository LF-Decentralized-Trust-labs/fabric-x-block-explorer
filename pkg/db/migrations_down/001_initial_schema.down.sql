-- Migration 001 rollback: drop all tables in reverse FK dependency order.
-- Run this only when you need to wipe and rebuild the schema from scratch.

DROP TABLE IF EXISTS tx_endorsements;
DROP TABLE IF EXISTS tx_blind_writes;
DROP TABLE IF EXISTS tx_read_writes;
DROP TABLE IF EXISTS tx_reads_only;
DROP TABLE IF EXISTS tx_namespaces;
DROP TABLE IF EXISTS block_envelope_errors;
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS namespace_policies;
DROP TABLE IF EXISTS blocks;
DROP TABLE IF EXISTS schema_migrations;
