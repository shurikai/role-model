-- name: CreateContributionDraft :one
INSERT INTO contribution_drafts (
    id, user_id, batch_id, employer_name, position_title,
    summary, full_description, outcomes, scale_context
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetContributionDraft :one
SELECT * FROM contribution_drafts
WHERE id = $1 AND user_id = $2;

-- name: ListContributionDraftsByBatch :many
SELECT * FROM contribution_drafts
WHERE batch_id = $1 AND user_id = $2
ORDER BY created_at;

-- name: UpdateContributionDraft :one
UPDATE contribution_drafts
SET summary          = $3,
    full_description  = $4,
    outcomes          = $5,
    scale_context     = $6,
    flags             = $7,
    updated_at        = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: UpdateContributionDraftStatus :one
UPDATE contribution_drafts
SET status     = $3,
    updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: CountContributionDraftsByBatch :one
SELECT
    COUNT(*) AS total,
    COUNT(*) FILTER (WHERE status = 'pending')  AS pending,
    COUNT(*) FILTER (WHERE status = 'approved') AS approved,
    COUNT(*) FILTER (WHERE status = 'rejected') AS rejected
FROM contribution_drafts
WHERE batch_id = $1 AND user_id = $2;
