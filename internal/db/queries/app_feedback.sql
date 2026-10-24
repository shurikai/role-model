-- name: CreateAppFeedback :one
INSERT INTO app_feedback (id, user_id, category, message, route, app_version, user_agent, viewport)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListAppFeedbackByStatus :many
-- For cmd/feedback, which triages across every account, so it is deliberately
-- not user-scoped. An empty status lists everything.
SELECT f.id, f.user_id, u.email, f.category, f.message, f.route, f.app_version,
       f.user_agent, f.viewport, f.status, f.created_at
FROM app_feedback f
JOIN users u ON u.id = f.user_id
WHERE sqlc.arg(status)::text = '' OR f.status = sqlc.arg(status)::text
ORDER BY f.created_at DESC, f.id;

-- name: MarkAppFeedbackTriaged :execrows
UPDATE app_feedback SET status = 'triaged'
WHERE id = $1;
