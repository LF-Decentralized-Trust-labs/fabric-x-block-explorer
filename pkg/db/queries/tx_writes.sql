-- name: GetWritesByTx :many
SELECT ns_id, key, value, is_blind_write, read_version
FROM tx_writes
WHERE block_num = $1 AND tx_num = $2
ORDER BY ns_id, key
LIMIT $3 OFFSET $4;
