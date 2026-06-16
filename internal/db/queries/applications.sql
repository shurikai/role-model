-- name: ListApplications :many
SELECT * FROM applications
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: GetApplication :one
SELECT * FROM applications
WHERE id = $1 AND user_id = $2;

-- name: CreateApplication :one
INSERT INTO applications (
    user_id, company_name, role_title, jd_url, jd_text, status, notes
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: UpdateApplication :one
UPDATE applications
SET company_name = $3,
    role_title   = $4,
    jd_url       = $5,
    jd_text      = $6,
    status       = $7,
    notes        = $8,
    updated_at   = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: UpdateApplicationSignals :one
UPDATE applications
SET jd_signals = $3,
    updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING *;
