-- name: InsertTxEndorsement :batchexec
INSERT INTO tx_endorsements (block_num, tx_num, ns_id, endorsement, msp_id, identity)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetEndorsementsByTx :many
SELECT ns_id, endorsement, msp_id, identity
FROM tx_endorsements
WHERE block_num = $1 AND tx_num = $2
ORDER BY ns_id, endorsement;
