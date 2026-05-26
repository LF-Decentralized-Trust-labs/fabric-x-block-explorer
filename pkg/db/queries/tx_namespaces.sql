-- name: GetNamespacesByTx :many
SELECT ns_id, ns_version
FROM tx_namespaces
WHERE block_num = $1 AND tx_num = $2
ORDER BY ns_id;

-- name: GetNamespacesByBlockTxRange :many
SELECT tx_num, ns_id, ns_version
FROM tx_namespaces
WHERE block_num = $1 AND tx_num >= $2 AND tx_num < $3
ORDER BY tx_num, ns_id;
