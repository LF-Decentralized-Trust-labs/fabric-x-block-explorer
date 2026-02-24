-- name: GetWritesByTx :many
SELECT 
    tw.id,
    tw.key,
    tw.value,
    tw.is_blind_write,
    tw.read_version,
    tn.ns_id
FROM tx_writes tw
JOIN tx_namespaces tn ON tw.tx_namespace_id = tn.id
JOIN transactions t ON tn.transaction_id = t.id
WHERE t.block_num = $1 AND t.tx_num = $2
ORDER BY tw.id
LIMIT $3 OFFSET $4;
