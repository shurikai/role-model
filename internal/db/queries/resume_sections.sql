-- name: ListResumeSectionsByUser :many
-- The user's section manifest in print order, hidden rows included.
--
-- Hidden rows are returned rather than filtered in SQL because the caller is
-- what decides they mean nothing to the document; a future settings screen
-- wants to list them, and a WHERE clause here would hide them from it too.
SELECT * FROM resume_sections
WHERE user_id = $1
ORDER BY sort_order, key;

-- name: CreateResumeSection :one
INSERT INTO resume_sections (
    id, user_id, key, heading, sort_order, hidden, source
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;
