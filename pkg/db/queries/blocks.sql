-- name: InsertBlock :exec
INSERT INTO blocks (block_num, tx_count, previous_hash, data_hash)
VALUES ($1, $2, $3, $4)
ON CONFLICT (block_num) DO NOTHING;

-- name: GetBlockHeight :one
SELECT COALESCE(MAX(block_num), 0) AS height
FROM blocks;

-- name: GetBlock :one
SELECT block_num, tx_count, previous_hash, data_hash
FROM blocks
WHERE block_num = $1;

-- name: ListBlocks :many
SELECT block_num, tx_count, previous_hash, data_hash
FROM blocks
WHERE block_num >= sqlc.arg(from_num) AND block_num <= sqlc.arg(to_num)
ORDER BY block_num
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);
