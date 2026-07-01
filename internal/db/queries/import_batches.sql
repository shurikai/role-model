-- name: CreateImportBatch :one
INSERT INTO import_batches (id, user_id, raw_text, status)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetImportBatch :one
SELECT * FROM import_batches
WHERE id = $1 AND user_id = $2;

-- name: ListImportBatches :many
SELECT * FROM import_batches
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: UpdateImportBatchStatus :one
UPDATE import_batches
SET status     = $3,
    error_text = $4,
    updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING *;
