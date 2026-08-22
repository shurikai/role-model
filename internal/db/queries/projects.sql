-- name: GetProjects :many
SELECT * FROM projects
WHERE user_id = $1
ORDER BY started_on DESC NULLS FIRST;

-- name: GetContributionsByProject :many
SELECT c.* FROM contributions c
JOIN project_contributions pc ON pc.contribution_id = c.id
WHERE pc.project_id = $1 AND c.user_id = $2 AND c.is_active = true
ORDER BY c.created_at;

-- name: GetTagsByProject :many
SELECT t.id, t.name, t.category, t.sort_order
FROM tags t
JOIN project_tags pt ON pt.tag_id = t.id
WHERE pt.project_id = $1 AND t.user_id = $2
ORDER BY t.sort_order, t.name;

-- name: GetProject :one
SELECT * FROM projects
WHERE id = $1 AND user_id = $2;

-- name: CreateProject :one
INSERT INTO projects (
    id, user_id, name, tagline, role, status, started_on, ended_on,
    source_url, live_url, writeup_url, force_include, force_exclude
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING *;

-- name: UpdateProject :one
UPDATE projects
SET name          = $3,
    tagline       = $4,
    role          = $5,
    status        = $6,
    started_on    = $7,
    ended_on      = $8,
    source_url      = $9,
    live_url      = $10,
    writeup_url   = $11,
    force_include = $12,
    force_exclude = $13,
    updated_at    = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: DeleteProject :execrows
DELETE FROM projects
WHERE id = $1 AND user_id = $2;

-- name: DeleteProjectContributions :exec
DELETE FROM project_contributions
WHERE project_id = $1;

-- name: DeleteProjectTags :exec
DELETE FROM project_tags
WHERE project_id = $1;

-- name: AssignContributionToProject :exec
INSERT INTO project_contributions (project_id, contribution_id)
VALUES ($1, $2)
ON CONFLICT (project_id, contribution_id) DO NOTHING;

-- name: UnassignContributionFromProject :execrows
DELETE FROM project_contributions
WHERE project_id = $1 AND contribution_id = $2;

-- name: AssignTagToProject :exec
INSERT INTO project_tags (project_id, tag_id)
VALUES ($1, $2)
ON CONFLICT (project_id, tag_id) DO NOTHING;

-- name: UnassignTagFromProject :execrows
DELETE FROM project_tags
WHERE project_id = $1 AND tag_id = $2;
