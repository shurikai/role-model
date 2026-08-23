-- name: CreateEntityDraft :one
INSERT INTO entity_drafts (
    id, user_id, batch_id, kind, payload, depends_on, flags, status
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetEntityDraft :one
SELECT * FROM entity_drafts
WHERE id = $1 AND user_id = $2;

-- name: ListEntityDraftsByBatch :many
-- Every draft in the batch, pending and resolved alike.
--
-- The resolver needs the resolved ones: a dependent draft finds its parent's
-- real id through resolved_id, and filtering them out here would make a
-- second approval pass unable to see what the first one created.
SELECT * FROM entity_drafts
WHERE batch_id = $1 AND user_id = $2
ORDER BY created_at, id;

-- name: MarkEntityDraftResolved :one
UPDATE entity_drafts
SET status = 'approved', resolved_id = $3, updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: MarkEntityDraftRejected :one
UPDATE entity_drafts
SET status = 'rejected', updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: SetEntityDraftFlags :one
UPDATE entity_drafts
SET flags = $3, updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING *;
