-- name: GetTagsByContribution :many
SELECT t.id, t.name, t.category, t.sort_order
FROM tags t
JOIN contribution_tags ct ON ct.tag_id = t.id
WHERE ct.contribution_id = $1 AND t.user_id = $2
ORDER BY t.sort_order, t.name;
