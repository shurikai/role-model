-- name: CreatePreference :one
INSERT INTO preferences (id, user_id, preference_type, label, sentiment, weight, context_type, notes)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetPreference :one
SELECT * FROM preferences
WHERE id = $1 AND user_id = $2;

-- name: ListPreferencesByUser :many
SELECT * FROM preferences
WHERE user_id = $1
ORDER BY preference_type, created_at;

-- name: ListPreferencesByUserAndType :many
SELECT * FROM preferences
WHERE user_id = $1 AND preference_type = $2
ORDER BY created_at;

-- name: ListHardExcludesByUser :many
SELECT * FROM preferences
WHERE user_id = $1 AND sentiment = 'hard_exclude'
ORDER BY preference_type, created_at;

-- name: UpdatePreference :one
UPDATE preferences
SET preference_type = $3,
    label           = $4,
    sentiment       = $5,
    weight          = $6,
    context_type    = $7,
    notes           = $8,
    updated_at      = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: DeletePreference :exec
DELETE FROM preferences
WHERE id = $1 AND user_id = $2;
