-- name: GetContributionsByPosition :many
SELECT * FROM contributions
WHERE position_id = $1 AND user_id = $2
ORDER BY created_at;

-- name: GetContribution :one
SELECT * FROM contributions
WHERE id = $1 AND user_id = $2;

-- name: CreateContribution :one
INSERT INTO contributions (
    id, user_id, position_id, summary, full_description, outcomes, scale_context, is_active
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateContribution :one
UPDATE contributions
SET summary         = $3,
    full_description = $4,
    outcomes        = $5,
    scale_context   = $6,
    is_active       = $7,
    updated_at      = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: DeleteContribution :exec
DELETE FROM contributions
WHERE id = $1 AND user_id = $2;

-- name: DeleteContributionTags :exec
DELETE FROM contribution_tags
WHERE contribution_id = $1;

-- name: DeleteContributionProjectLinks :exec
DELETE FROM project_contributions
WHERE contribution_id = $1;
