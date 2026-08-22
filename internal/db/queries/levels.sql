-- name: ListCareerLevelsByUser :many
-- The user's seniority ladder, weakest first.
--
-- One query serves three readers: the length budget and the framing guidance
-- the 2a prompt is given, and the enum the extraction prompt is told to choose
-- a posting's seniority from. They read the same rows on purpose -- the six
-- hand-synced copies this table replaced had already drifted apart, leaving
-- three levels that the database accepted and extraction could never emit.
SELECT * FROM career_levels
WHERE user_id = $1
ORDER BY rank, value;

-- name: ListProficiencyLevelsByUser :many
-- The user's depth scale, weakest first. Read by the fit gate to compare a
-- skill's recorded proficiency against the depth a posting asked for.
SELECT * FROM proficiency_levels
WHERE user_id = $1
ORDER BY rank, value;

-- name: CreateCareerLevel :one
INSERT INTO career_levels (
    id, user_id, value, label, rank, aliases,
    length_budget, framing_guidance, is_fallback, source, sort_order
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: CreateProficiencyLevel :one
INSERT INTO proficiency_levels (
    id, user_id, value, label, rank, aliases, source, sort_order
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;
