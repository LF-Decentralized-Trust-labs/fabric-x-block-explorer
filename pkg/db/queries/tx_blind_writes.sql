-- name: InsertBlindWrite :batchexec
INSERT INTO tx_blind_writes (block_num, tx_num, ns_id, key, value)
VALUES ($1, $2, $3, $4, $5);

-- name: GetBlindWritesByTx :many
SELECT ns_id, key, value
FROM tx_blind_writes
WHERE block_num = $1 AND tx_num = $2
ORDER BY ns_id, key
LIMIT $3 OFFSET $4;
