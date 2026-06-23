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
