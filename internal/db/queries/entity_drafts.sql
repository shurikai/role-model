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
-- Rejecting is legal only from pending.
--
-- The guard is in the WHERE clause rather than in a Go check before the call,
-- because a check-then-write loses to a resolve that lands between the two:
-- the draft becomes 'approved' with a real row behind it, and the reject
-- overwrites the status while the row it created stays. What the caller sees
-- for a non-pending draft is pgx.ErrNoRows, which is the same thing it sees
-- for a draft belonging to someone else -- so a caller that needs to tell 404
-- from 409 reads the draft first and uses this as the authority.
UPDATE entity_drafts
SET status = 'rejected', updated_at = now()
WHERE id = $1 AND user_id = $2 AND status = 'pending'
RETURNING *;

-- name: UpdateEntityDraftPayload :one
-- Full-payload replace, pending only.
--
-- Not a field-level patch: the five kinds have five payload shapes, and the
-- editor for a kind always submits a complete object for it. The same
-- pending-only guard as MarkEntityDraftRejected, for the same race.
UPDATE entity_drafts
SET payload = $3, updated_at = now()
WHERE id = $1 AND user_id = $2 AND status = 'pending'
RETURNING *;

-- name: SetEntityDraftFlags :one
UPDATE entity_drafts
SET flags = $3, updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING *;
