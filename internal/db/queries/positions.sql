-- name: GetPositionsByEmployer :many
SELECT * FROM positions
WHERE employer_id = $1 AND user_id = $2
ORDER BY sort_order, started_on DESC;

-- name: GetPosition :one
SELECT * FROM positions
WHERE id = $1 AND user_id = $2;

-- name: CreatePosition :one
INSERT INTO positions (
    id, user_id, employer_id, title, industry_level, industry_role,
    location, level_rationale, started_on, ended_on, context_narrative, sort_order
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: UpdatePosition :one
UPDATE positions
SET title             = $3,
    industry_level    = $4,
    industry_role     = $5,
    location          = $6,
    level_rationale   = $7,
    started_on        = $8,
    ended_on          = $9,
    context_narrative = $10,
    sort_order        = $11,
    updated_at        = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: DeletePosition :exec
DELETE FROM positions
WHERE id = $1 AND user_id = $2;

-- name: CountContributionsByPosition :one
SELECT count(*) FROM contributions
WHERE position_id = $1 AND user_id = $2;
