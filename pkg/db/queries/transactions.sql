-- name: InsertTransaction :batchexec
INSERT INTO transactions (block_num, tx_num, tx_id, validation_code)
VALUES ($1, $2, $3, $4)
ON CONFLICT (block_num, tx_num) DO UPDATE SET tx_id = EXCLUDED.tx_id;

-- name: GetValidationCodeByBlock :many
SELECT block_num, tx_num, tx_id, validation_code
FROM transactions
WHERE block_num = $1
ORDER BY tx_num
LIMIT $2 OFFSET $3;

-- name: GetValidationCodeByTxID :one
SELECT block_num, tx_num, tx_id, validation_code
FROM transactions
WHERE tx_id = $1;

-- name: InsertTxNamespace :batchexec
INSERT INTO tx_namespaces (block_num, tx_num, ns_id, ns_version)
VALUES ($1, $2, $3, $4)
ON CONFLICT (block_num, tx_num, ns_id) DO UPDATE SET ns_version = EXCLUDED.ns_version;

-- name: InsertTxRead :batchexec
INSERT INTO tx_reads (block_num, tx_num, ns_id, key, version, is_read_write)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: InsertTxWrite :batchexec
INSERT INTO tx_writes (block_num, tx_num, ns_id, key, value, is_blind_write, read_version)
VALUES ($1, $2, $3, $4, $5, $6, $7);
