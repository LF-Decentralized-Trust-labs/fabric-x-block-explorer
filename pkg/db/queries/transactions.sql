-- name: InsertTransaction :batchexec
INSERT INTO transactions (
    block_num, tx_num, tx_id, validation_code, tx_type, chaincode_name,
    creator_msp_id, creator_id_bytes, creator_nonce, envelope_signature,
    payload_extension, channel_version, channel_id, epoch, tls_cert_hash,
    created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
ON CONFLICT (block_num, tx_num) DO NOTHING;

-- name: GetValidationCodeByBlock :many
SELECT
    block_num, tx_num, tx_id, validation_code, tx_type, chaincode_name,
    creator_msp_id, creator_id_bytes, creator_nonce, envelope_signature,
    payload_extension, channel_version, channel_id, epoch, tls_cert_hash,
    created_at
FROM transactions
WHERE block_num = $1
ORDER BY tx_num
LIMIT $2 OFFSET $3;

-- name: GetValidationCodeByTxID :one
SELECT
    block_num, tx_num, tx_id, validation_code, tx_type, chaincode_name,
    creator_msp_id, creator_id_bytes, creator_nonce, envelope_signature,
    payload_extension, channel_version, channel_id, epoch, tls_cert_hash,
    created_at
FROM transactions
WHERE tx_id = $1;

-- name: InsertTxNamespace :batchexec
INSERT INTO tx_namespaces (block_num, tx_num, ns_id, ns_version)
VALUES ($1, $2, $3, $4)
ON CONFLICT (block_num, tx_num, ns_id) DO NOTHING;
