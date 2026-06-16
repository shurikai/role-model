-- name: GetEmployers :many
SELECT * FROM employers
WHERE user_id = $1
ORDER BY name;

-- name: GetEmployer :one
SELECT * FROM employers
WHERE id = $1 AND user_id = $2;
