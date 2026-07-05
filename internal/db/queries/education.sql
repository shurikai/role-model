-- name: GetEducation :many
SELECT * FROM education
WHERE user_id = $1
ORDER BY ended_on DESC NULLS FIRST;

-- name: CreateEducation :one
INSERT INTO education (id, user_id, institution, degree, field_of_study, started_on, ended_on, notes)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateEducation :one
UPDATE education
SET institution    = $3,
    degree         = $4,
    field_of_study = $5,
    started_on     = $6,
    ended_on       = $7,
    notes          = $8,
    updated_at     = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: DeleteEducation :execrows
DELETE FROM education
WHERE id = $1 AND user_id = $2;
