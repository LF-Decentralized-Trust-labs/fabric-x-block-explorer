CREATE TABLE IF NOT EXISTS blocks (
    block_num BIGINT PRIMARY KEY,
    tx_count INT NOT NULL,
    previous_hash BYTEA,
    data_hash BYTEA
);

-- Natural composite PK; no synthetic surrogate needed.
CREATE TABLE IF NOT EXISTS transactions (
    block_num BIGINT NOT NULL REFERENCES blocks(block_num),
    tx_num    BIGINT NOT NULL,
    tx_id     BYTEA  NOT NULL,
    validation_code BIGINT NOT NULL,
    PRIMARY KEY (block_num, tx_num)
);

-- Natural composite PK; FK references transactions' natural key.
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
    key        BYTEA  NOT NULL,
    version    BIGINT,
    PRIMARY KEY (block_num, tx_num, ns_id, key),
    FOREIGN KEY (block_num, tx_num, ns_id) REFERENCES tx_namespaces(block_num, tx_num, ns_id)
);

-- Keys that were both read and written. From ns.ReadWrites in the block.
CREATE TABLE IF NOT EXISTS tx_read_writes (
    block_num    BIGINT NOT NULL,
    tx_num       BIGINT NOT NULL,
    ns_id        TEXT   NOT NULL,
    key          BYTEA  NOT NULL,
    read_version BIGINT,
    value        BYTEA,
    PRIMARY KEY (block_num, tx_num, ns_id, key),
    FOREIGN KEY (block_num, tx_num, ns_id) REFERENCES tx_namespaces(block_num, tx_num, ns_id)
);

-- Keys that were written without a prior read. From ns.BlindWrites in the block.
CREATE TABLE IF NOT EXISTS tx_blind_writes (
    block_num  BIGINT NOT NULL,
    tx_num     BIGINT NOT NULL,
    ns_id      TEXT   NOT NULL,
    key        BYTEA  NOT NULL,
    value      BYTEA,
    PRIMARY KEY (block_num, tx_num, ns_id, key),
    FOREIGN KEY (block_num, tx_num, ns_id) REFERENCES tx_namespaces(block_num, tx_num, ns_id)
);

CREATE TABLE IF NOT EXISTS tx_endorsements (
    block_num   BIGINT NOT NULL,
    tx_num      BIGINT NOT NULL,
    ns_id       TEXT   NOT NULL,
    endorsement BYTEA  NOT NULL,
    msp_id      TEXT,
    identity    JSONB,
    PRIMARY KEY (block_num, tx_num, ns_id, endorsement),
    FOREIGN KEY (block_num, tx_num, ns_id) REFERENCES tx_namespaces(block_num, tx_num, ns_id)
);

-- Natural composite PK; (namespace, version) is already unique.
CREATE TABLE IF NOT EXISTS namespace_policies (
    namespace TEXT   NOT NULL,
    version   BIGINT NOT NULL,
    policy    JSONB,
    PRIMARY KEY (namespace, version)
);

-- Indexes to improve lookup performance.
CREATE INDEX IF NOT EXISTS idx_transactions_block_num          ON transactions(block_num);
CREATE INDEX IF NOT EXISTS idx_tx_namespaces_block_tx          ON tx_namespaces(block_num, tx_num);
CREATE INDEX IF NOT EXISTS idx_tx_reads_only_block_tx_ns       ON tx_reads_only(block_num, tx_num, ns_id);
CREATE INDEX IF NOT EXISTS idx_tx_read_writes_block_tx_ns      ON tx_read_writes(block_num, tx_num, ns_id);
CREATE INDEX IF NOT EXISTS idx_tx_blind_writes_block_tx_ns     ON tx_blind_writes(block_num, tx_num, ns_id);
CREATE INDEX IF NOT EXISTS idx_tx_endorsements_block_tx_ns     ON tx_endorsements(block_num, tx_num, ns_id);
CREATE INDEX IF NOT EXISTS idx_namespace_policies_namespace    ON namespace_policies(namespace);
