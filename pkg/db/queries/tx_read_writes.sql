-- name: InsertReadWrite :batchexec
INSERT INTO tx_read_writes (block_num, tx_num, ns_id, key, read_version, value)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (block_num, tx_num, ns_id, key) DO NOTHING;

-- name: GetReadWritesByTx :many
SELECT ns_id, key, read_version, value
FROM tx_read_writes
WHERE block_num = $1 AND tx_num = $2
ORDER BY ns_id, key;

-- name: GetReadWritesByBlockTxRange :many
SELECT tx_num, ns_id, key, read_version, value
FROM tx_read_writes
WHERE block_num = $1 AND tx_num >= $2 AND tx_num < $3
ORDER BY tx_num, ns_id, key;
