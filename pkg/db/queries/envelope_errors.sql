-- name: InsertEnvelopeError :batchexec
INSERT INTO block_envelope_errors (block_num, tx_num, validation_code, raw_envelope, tx_id)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (block_num, tx_num) DO NOTHING;

-- name: GetEnvelopeErrorsByBlock :many
SELECT block_num, tx_num, validation_code, raw_envelope, tx_id
FROM block_envelope_errors
WHERE block_num = $1
ORDER BY tx_num;
