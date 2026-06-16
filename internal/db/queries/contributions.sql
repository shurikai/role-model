-- name: GetContributionsByPosition :many
SELECT * FROM contributions
WHERE position_id = $1 AND user_id = $2
ORDER BY created_at;

-- name: GetContribution :one
SELECT * FROM contributions
WHERE id = $1 AND user_id = $2;
