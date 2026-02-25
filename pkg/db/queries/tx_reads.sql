-- name: GetReadsByTx :many
SELECT ns_id, key, version, is_read_write
FROM tx_reads
WHERE block_num = $1 AND tx_num = $2
ORDER BY ns_id, key;
