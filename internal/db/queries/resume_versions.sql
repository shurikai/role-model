-- name: NextResumeVersionNumber :one
SELECT COALESCE(MAX(version_number), 0) + 1
FROM resume_versions
WHERE application_id = $1 AND user_id = $2;

-- name: CreateResumeVersion :one
INSERT INTO resume_versions (
    id, user_id, application_id, version_number, generation_params, structured_output
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: ListResumeVersions :many
SELECT * FROM resume_versions
WHERE application_id = $1 AND user_id = $2
ORDER BY version_number DESC;

-- name: GetResumeVersion :one
SELECT * FROM resume_versions
WHERE id = $1 AND user_id = $2;
