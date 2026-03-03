-- name: InsertReadOnly :batchexec
INSERT INTO tx_reads_only (block_num, tx_num, ns_id, key, version)
VALUES ($1, $2, $3, $4, $5);

-- name: GetReadsOnlyByTx :many
SELECT ns_id, key, version
FROM tx_reads_only
WHERE block_num = $1 AND tx_num = $2
ORDER BY ns_id, key;
