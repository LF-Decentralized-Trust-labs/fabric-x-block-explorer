-- name: GetReadsByTx :many
SELECT tr.id, tr.key, tr.version, tr.is_read_write, tn.ns_id
FROM tx_reads tr
JOIN tx_namespaces tn ON tr.tx_namespace_id = tn.id
JOIN transactions t ON tn.transaction_id = t.id
WHERE t.block_num = $1 AND t.tx_num = $2
ORDER BY tr.id;
