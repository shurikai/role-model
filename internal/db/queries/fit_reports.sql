-- name: CreateFitReport :one
INSERT INTO fit_reports (
    id, user_id, application_id, dealbreakers_clear, dealbreaker_hits,
    capability_score, capability_gaps, preference_gaps, narrative,
    preference_conflicts, screening_summary, capability_matches, preference_matches,
    capability_partial
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
)
RETURNING *;

-- name: GetFitReport :one
SELECT * FROM fit_reports
WHERE id = $1 AND user_id = $2;

-- name: ListFitReportsByApplication :many
SELECT * FROM fit_reports
WHERE application_id = $1 AND user_id = $2
ORDER BY created_at DESC;

-- name: ListFitReports :many
SELECT * FROM fit_reports
WHERE user_id = $1
ORDER BY created_at DESC;
