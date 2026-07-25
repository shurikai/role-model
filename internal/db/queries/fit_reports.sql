-- name: CreateFitReport :one
INSERT INTO fit_reports (
    id, user_id, application_id, anti_pattern_passed, anti_pattern_hits,
    technical_score, technical_gaps, preference_score, preference_gaps, narrative,
    preference_conflicts
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
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
