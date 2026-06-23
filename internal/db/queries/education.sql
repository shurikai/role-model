-- name: GetEducation :many
SELECT * FROM education
WHERE user_id = $1
ORDER BY ended_on DESC NULLS FIRST;
