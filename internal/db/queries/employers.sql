-- name: GetEmployers :many
SELECT * FROM employers
WHERE user_id = $1
ORDER BY name;

-- name: GetEmployer :one
SELECT * FROM employers
WHERE id = $1 AND user_id = $2;

-- name: CreateEmployer :one
INSERT INTO employers (id, user_id, name, industry, notes)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateEmployer :one
UPDATE employers
SET name     = $3,
    industry = $4,
    notes    = $5,
    updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: DeleteEmployer :exec
DELETE FROM employers
WHERE id = $1 AND user_id = $2;

-- name: CountPositionsByEmployer :one
SELECT count(*) FROM positions
WHERE employer_id = $1 AND user_id = $2;
