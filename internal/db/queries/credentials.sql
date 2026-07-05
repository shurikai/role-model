-- name: GetCredentials :many
SELECT * FROM credentials
WHERE user_id = $1
ORDER BY issued_on DESC NULLS FIRST;

-- name: CreateCredential :one
INSERT INTO credentials (id, user_id, name, issuer, issued_on, expires_on, credential_url)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UpdateCredential :one
UPDATE credentials
SET name           = $3,
    issuer         = $4,
    issued_on      = $5,
    expires_on     = $6,
    credential_url = $7,
    updated_at     = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: DeleteCredential :execrows
DELETE FROM credentials
WHERE id = $1 AND user_id = $2;

-- name: DeleteContributionFeedback :exec
DELETE FROM contribution_feedback
WHERE contribution_id = $1;
