-- name: ListEmployersByRecency :many
-- The career view's employer order: resume order, not alphabetical. An
-- employer with a current position (ended_on IS NULL) comes first, then by
-- the latest end date, then the latest start. An employer with no positions
-- has nothing to date it by and sorts last.
SELECT e.*
FROM employers e
LEFT JOIN LATERAL (
    SELECT bool_or(p.ended_on IS NULL) AS has_current,
           max(p.ended_on)             AS last_ended,
           max(p.started_on)           AS last_started
    FROM positions p
    WHERE p.employer_id = e.id AND p.user_id = e.user_id
) r ON true
WHERE e.user_id = $1
ORDER BY r.has_current DESC NULLS LAST,
         r.last_ended DESC NULLS LAST,
         r.last_started DESC NULLS LAST,
         e.name, e.id;

-- name: ListPositionsByUser :many
SELECT * FROM positions
WHERE user_id = $1
ORDER BY sort_order, started_on DESC, id;

-- name: CountContributionsByPositionForUser :many
-- Contributions with no position (project-only) belong to no position and are
-- not counted here.
SELECT position_id::uuid AS position_id,
       (count(*) FILTER (WHERE is_active))::int     AS active,
       (count(*) FILTER (WHERE NOT is_active))::int AS hidden
FROM contributions
WHERE user_id = $1 AND position_id IS NOT NULL
GROUP BY position_id;

-- name: ListContributionTagsByPosition :many
-- Every tag on every contribution of one position, so the position page is
-- one request for contributions and their tags rather than one per card.
SELECT ct.contribution_id, t.id, t.name, t.category
FROM contribution_tags ct
JOIN contributions c ON c.id = ct.contribution_id
JOIN tags t ON t.id = ct.tag_id
WHERE c.position_id = $1 AND c.user_id = $2 AND t.user_id = $2
ORDER BY t.sort_order, t.name;
