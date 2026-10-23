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

-- name: ListImportBatchSummaries :many
-- The hub's list of imports. kind is derived, not stored: the narrow path
-- writes only contribution_drafts and the career path only entity_drafts, so
-- a batch with any entity draft is a career batch. That holds only once drafts
-- exist -- a career batch that failed before staging anything reads as
-- 'contributions' -- which is why the hub lists only batches with pending
-- drafts. raw_text is deliberately not selected; a list has no use for it.
SELECT
    b.id,
    b.status,
    b.created_at,
    (CASE WHEN EXISTS (SELECT 1 FROM entity_drafts e WHERE e.batch_id = b.id)
          THEN 'career' ELSE 'contributions' END)::text AS kind,
    ((SELECT COUNT(*) FROM entity_drafts e
       WHERE e.batch_id = b.id AND e.status = 'pending')
     + (SELECT COUNT(*) FROM contribution_drafts c
         WHERE c.batch_id = b.id AND c.status = 'pending'))::int AS pending_count
FROM import_batches b
WHERE b.user_id = $1
ORDER BY b.created_at DESC, b.id;
