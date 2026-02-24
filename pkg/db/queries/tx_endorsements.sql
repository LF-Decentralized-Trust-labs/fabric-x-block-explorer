-- name: InsertTxEndorsement :exec
INSERT INTO tx_endorsements (tx_namespace_id, endorsement, msp_id, identity)
VALUES ($1, $2, $3, $4);

-- name: GetEndorsementsByTx :many
SELECT te.id, te.endorsement, te.msp_id, te.identity, tn.ns_id
FROM tx_endorsements te
JOIN tx_namespaces tn ON te.tx_namespace_id = tn.id
JOIN transactions t ON tn.transaction_id = t.id
WHERE t.block_num = $1 AND t.tx_num = $2
ORDER BY te.id;
