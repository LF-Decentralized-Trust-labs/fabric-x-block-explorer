-- Migration 001: initial schema
-- Creates all tables required by the block explorer.
-- This is the single canonical migration; schema.sql and this file are kept in sync.

CREATE TABLE IF NOT EXISTS blocks (
    block_num           BIGINT PRIMARY KEY,
    -- tx_count is the number of transactions persisted from this block.
    -- It excludes envelopes that failed to unmarshal, config transactions
    -- (which are stored in namespace_policies instead), and envelopes with
    -- no parseable tx_id (MALFORMED_BAD_ENVELOPE, MALFORMED_MISSING_TX_ID,
    -- REJECTED_DUPLICATE_TX_ID). All other transactions — including COMMITTED
    -- ones with corrupt rwsets and all ABORTED/MALFORMED ones that have a
    -- tx_id — are stored as minimal records.
    -- It does NOT equal the raw envelope count in the Fabric block.
    tx_count            INT NOT NULL,
    previous_hash       BYTEA,
    data_hash           BYTEA,
    block_size          INT,
    created_at          TIMESTAMP,
    -- Block-level orderer signatures (raw bytes from BlockMetadata[0] / SIGNATURES).
    metadata_signatures BYTEA,
    -- Index of the last config block as decoded from BlockMetadata[1] / LAST_CONFIG.
    last_config_index   BIGINT,
    -- Raw per-tx validation-code byte array from BlockMetadata[2] / TRANSACTIONS_FILTER.
    tx_status_codes     BYTEA,
    -- Commit hash from BlockMetadata[3] / COMMIT_HASH.
    commit_hash         BYTEA
);

CREATE TABLE IF NOT EXISTS transactions (
    block_num               BIGINT NOT NULL REFERENCES blocks(block_num),
    tx_num                  BIGINT NOT NULL,
    tx_id                   BYTEA  NOT NULL,
    validation_code         TEXT NOT NULL,
    tx_type                 TEXT,
    chaincode_name          TEXT,
    creator_msp_id          TEXT,
    creator_id_bytes        BYTEA,
    creator_nonce           BYTEA,
    envelope_signature      BYTEA,
    payload_extension       BYTEA,
    channel_version         INT,
    channel_id              TEXT,
    epoch                   BIGINT,
    tls_cert_hash           BYTEA,
    created_at              TIMESTAMP,
    PRIMARY KEY (block_num, tx_num)
);

CREATE TABLE IF NOT EXISTS tx_namespaces (
    block_num  BIGINT NOT NULL,
    tx_num     BIGINT NOT NULL,
    ns_id      TEXT   NOT NULL,
    ns_version BIGINT NOT NULL,
    PRIMARY KEY (block_num, tx_num, ns_id),
    FOREIGN KEY (block_num, tx_num) REFERENCES transactions(block_num, tx_num)
);

-- Keys that were only read (no write). From ns.ReadsOnly in the block.
CREATE TABLE IF NOT EXISTS tx_reads_only (
    block_num  BIGINT NOT NULL,
    tx_num     BIGINT NOT NULL,
    ns_id      TEXT   NOT NULL,
    seq_num    INT    NOT NULL,
    key        BYTEA  NOT NULL,
    version    BIGINT,
    PRIMARY KEY (block_num, tx_num, ns_id, seq_num),
    FOREIGN KEY (block_num, tx_num, ns_id) REFERENCES tx_namespaces(block_num, tx_num, ns_id)
);

-- Keys that were both read and written. From ns.ReadWrites in the block.
CREATE TABLE IF NOT EXISTS tx_read_writes (
    block_num    BIGINT NOT NULL,
    tx_num       BIGINT NOT NULL,
    ns_id        TEXT   NOT NULL,
    seq_num      INT    NOT NULL,
    key          BYTEA  NOT NULL,
    read_version BIGINT,
    value        BYTEA,
    PRIMARY KEY (block_num, tx_num, ns_id, seq_num),
    FOREIGN KEY (block_num, tx_num, ns_id) REFERENCES tx_namespaces(block_num, tx_num, ns_id)
);

-- Keys that were written without a prior read. From ns.BlindWrites in the block.
CREATE TABLE IF NOT EXISTS tx_blind_writes (
    block_num  BIGINT NOT NULL,
    tx_num     BIGINT NOT NULL,
    ns_id      TEXT   NOT NULL,
    seq_num    INT    NOT NULL,
    key        BYTEA  NOT NULL,
    value      BYTEA,
    PRIMARY KEY (block_num, tx_num, ns_id, seq_num),
    FOREIGN KEY (block_num, tx_num, ns_id) REFERENCES tx_namespaces(block_num, tx_num, ns_id)
);

CREATE TABLE IF NOT EXISTS tx_endorsements (
    block_num   BIGINT NOT NULL,
    tx_num      BIGINT NOT NULL,
    ns_id       TEXT   NOT NULL,
    seq_num     INT    NOT NULL,
    endorsement BYTEA  NOT NULL,
    msp_id      TEXT,
    identity    JSONB,
    PRIMARY KEY (block_num, tx_num, ns_id, seq_num),
    FOREIGN KEY (block_num, tx_num, ns_id) REFERENCES tx_namespaces(block_num, tx_num, ns_id)
);

CREATE TABLE IF NOT EXISTS namespace_policies (
    namespace TEXT   NOT NULL,
    version   BIGINT NOT NULL,
    policy    JSONB,
    PRIMARY KEY (namespace, version)
);

-- Indexes to improve lookup performance.
CREATE INDEX IF NOT EXISTS idx_namespace_policies_namespace ON namespace_policies(namespace);

-- Envelopes that could not be stored as regular transactions because they lacked
-- a parseable tx_id or were flagged as duplicates by the committer.
-- Every block position (block_num, tx_num) that is NOT in `transactions` and NOT
-- a config envelope will have a row here, ensuring full block coverage for the UI.
CREATE TABLE IF NOT EXISTS block_envelope_errors (
    block_num       BIGINT NOT NULL REFERENCES blocks(block_num),
    tx_num          BIGINT NOT NULL,
    -- validation_code mirrors the committer-assigned status (e.g. MALFORMED_BAD_ENVELOPE).
    validation_code TEXT   NOT NULL,
    -- raw_envelope preserves the original protobuf bytes from block.Data.Data[tx_num].
    raw_envelope    BYTEA,
    -- tx_id is populated only when a tx_id could be extracted (e.g. REJECTED_DUPLICATE_TX_ID).
    tx_id           BYTEA,
    PRIMARY KEY (block_num, tx_num)
);

CREATE INDEX IF NOT EXISTS idx_block_envelope_errors_block_num ON block_envelope_errors(block_num);
