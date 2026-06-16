-- name: GetPositionsByEmployer :many
SELECT * FROM positions
WHERE employer_id = $1 AND user_id = $2
ORDER BY sort_order, started_on DESC;

-- name: GetPosition :one
SELECT * FROM positions
WHERE id = $1 AND user_id = $2;
