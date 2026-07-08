-- name: GetTagsByContribution :many
SELECT t.id, t.name, t.category, t.sort_order
FROM tags t
JOIN contribution_tags ct ON ct.tag_id = t.id
WHERE ct.contribution_id = $1 AND t.user_id = $2
ORDER BY t.sort_order, t.name;

-- name: CreateTagCategory :one
INSERT INTO tag_categories (id, user_id, name, sort_order)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListTagCategories :many
SELECT * FROM tag_categories
WHERE user_id = $1
ORDER BY sort_order, name;

-- name: DeleteTagCategory :execrows
DELETE FROM tag_categories
WHERE id = $1 AND user_id = $2;

-- name: CountTagsInCategory :one
SELECT count(*) FROM tags
WHERE user_id = $1 AND category = $2;

-- name: CreateTag :one
INSERT INTO tags (id, user_id, name, aliases, category, sort_order)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListTags :many
SELECT * FROM tags
WHERE user_id = $1
ORDER BY sort_order, name;

-- name: GetTag :one
SELECT * FROM tags
WHERE id = $1 AND user_id = $2;

-- name: DeleteTag :execrows
DELETE FROM tags
WHERE id = $1 AND user_id = $2;

-- name: CountTagUsage :one
SELECT
  (SELECT count(*) FROM contribution_tags ct WHERE ct.tag_id = $1) +
  (SELECT count(*) FROM project_tags pt WHERE pt.tag_id = $1) AS total;

-- name: AssignTagToContribution :exec
INSERT INTO contribution_tags (contribution_id, tag_id)
VALUES ($1, $2)
ON CONFLICT (contribution_id, tag_id) DO NOTHING;

-- name: UnassignTagFromContribution :execrows
DELETE FROM contribution_tags
WHERE contribution_id = $1 AND tag_id = $2;
