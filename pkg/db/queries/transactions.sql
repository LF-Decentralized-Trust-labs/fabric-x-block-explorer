-- name: InsertTransaction :one
INSERT INTO transactions (block_num, tx_num, tx_id, validation_code)
VALUES ($1, $2, $3, $4)
ON CONFLICT (block_num, tx_num) DO UPDATE SET tx_id = EXCLUDED.tx_id
RETURNING id;

-- name: GetValidationCodeByBlock :many
SELECT id, block_num, tx_num, tx_id, validation_code
FROM transactions
WHERE block_num = $1
ORDER BY tx_num
LIMIT $2 OFFSET $3;

-- name: GetValidationCodeByTxID :one
SELECT id, block_num, tx_num, tx_id, validation_code
FROM transactions
WHERE tx_id = $1;

-- name: GetTransactionID :one
SELECT id
FROM transactions
WHERE block_num = $1 AND tx_num = $2;

-- name: InsertTxNamespace :one
INSERT INTO tx_namespaces (transaction_id, ns_id, ns_version)
VALUES ($1, $2, $3)
ON CONFLICT (transaction_id, ns_id) DO UPDATE SET ns_version = EXCLUDED.ns_version
RETURNING id;

-- name: InsertTxRead :exec
INSERT INTO tx_reads (tx_namespace_id, key, version, is_read_write)
VALUES ($1, $2, $3, $4);

-- name: InsertTxWrite :exec
INSERT INTO tx_writes (tx_namespace_id, key, value, is_blind_write, read_version)
VALUES ($1, $2, $3, $4, $5);
