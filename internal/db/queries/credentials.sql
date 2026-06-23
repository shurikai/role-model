-- name: GetCredentials :many
SELECT * FROM credentials
WHERE user_id = $1
ORDER BY issued_on DESC NULLS FIRST;
