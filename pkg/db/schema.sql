CREATE TABLE IF NOT EXISTS blocks (
    block_num BIGINT PRIMARY KEY,
    tx_count INT NOT NULL,
    previous_hash BYTEA,
    data_hash BYTEA
);

CREATE TABLE IF NOT EXISTS transactions (
    id BIGSERIAL PRIMARY KEY,
    block_num BIGINT NOT NULL REFERENCES blocks(block_num),
    tx_num BIGINT NOT NULL,
    tx_id BYTEA NOT NULL,
    validation_code BIGINT NOT NULL,  
    UNIQUE (block_num, tx_num)
);

CREATE TABLE IF NOT EXISTS tx_namespaces (
    id BIGSERIAL PRIMARY KEY,
    transaction_id BIGINT NOT NULL REFERENCES transactions(id),
    ns_id TEXT NOT NULL,
    ns_version BIGINT NOT NULL,
    UNIQUE (transaction_id, ns_id)
);

CREATE TABLE IF NOT EXISTS tx_reads (
    id BIGSERIAL PRIMARY KEY,
    tx_namespace_id BIGINT NOT NULL REFERENCES tx_namespaces(id),
    key BYTEA NOT NULL,
    version BIGINT,
    is_read_write BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS tx_writes (
    id BIGSERIAL PRIMARY KEY,
    tx_namespace_id BIGINT NOT NULL REFERENCES tx_namespaces(id),
    key BYTEA NOT NULL,
    value BYTEA,
    is_blind_write BOOLEAN NOT NULL DEFAULT FALSE,
    read_version BIGINT
);

CREATE TABLE IF NOT EXISTS tx_endorsements (
    id BIGSERIAL PRIMARY KEY,
    tx_namespace_id BIGINT NOT NULL REFERENCES tx_namespaces(id),
    endorsement BYTEA NOT NULL,
    msp_id TEXT,
    identity JSONB
);

CREATE TABLE IF NOT EXISTS namespace_policies (
    id BIGSERIAL PRIMARY KEY,
    namespace TEXT NOT NULL,
    version BIGINT NOT NULL,
    policy JSONB,
    UNIQUE (namespace, version)
);

-- Indexes for foreign key columns to improve JOIN performance
CREATE INDEX IF NOT EXISTS idx_transactions_block_num ON transactions(block_num);
CREATE INDEX IF NOT EXISTS idx_tx_namespaces_transaction_id ON tx_namespaces(transaction_id);
CREATE INDEX IF NOT EXISTS idx_tx_reads_tx_namespace_id ON tx_reads(tx_namespace_id);
CREATE INDEX IF NOT EXISTS idx_tx_writes_tx_namespace_id ON tx_writes(tx_namespace_id);
CREATE INDEX IF NOT EXISTS idx_tx_endorsements_tx_namespace_id ON tx_endorsements(tx_namespace_id);
CREATE INDEX IF NOT EXISTS idx_namespace_policies_namespace ON namespace_policies(namespace);
