-- name: CreatePreference :one
INSERT INTO preferences (id, user_id, preference_type, label, sentiment, weight, is_hard_gate, context_type, notes, aliases)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
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

-- name: ListHardGatesByUser :many
SELECT * FROM preferences
WHERE user_id = $1 AND is_hard_gate
ORDER BY preference_type, created_at;

-- name: UpdatePreference :one
UPDATE preferences
SET preference_type = $3,
    label           = $4,
    sentiment       = $5,
    weight          = $6,
    is_hard_gate    = $7,
    context_type    = $8,
    notes           = $9,
    aliases         = $10,
    updated_at      = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: DeletePreference :execrows
DELETE FROM preferences
WHERE id = $1 AND user_id = $2;
