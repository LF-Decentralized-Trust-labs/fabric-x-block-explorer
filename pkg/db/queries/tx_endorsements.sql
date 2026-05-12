-- name: InsertTxEndorsement :batchexec
INSERT INTO tx_endorsements (block_num, tx_num, ns_id, seq_num, endorsement, msp_id, identity)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (block_num, tx_num, ns_id, seq_num) DO NOTHING;

-- name: GetEndorsementsByTx :many
SELECT ns_id, endorsement, msp_id, identity
FROM tx_endorsements
WHERE block_num = $1 AND tx_num = $2
ORDER BY ns_id, seq_num;

-- name: GetEndorsementsByBlockTxRange :many
SELECT tx_num, ns_id, endorsement, msp_id, identity
FROM tx_endorsements
WHERE block_num = $1 AND tx_num >= $2 AND tx_num < $3
ORDER BY tx_num, ns_id, seq_num;
