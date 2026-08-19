-- name: CreateSkill :one
INSERT INTO skills (id, user_id, tag_id, proficiency, years_experience, is_active)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetSkill :one
SELECT * FROM skills
WHERE id = $1 AND user_id = $2;

-- name: ListSkillsByUser :many
SELECT * FROM skills
WHERE user_id = $1
ORDER BY created_at;

-- name: ListActiveSkillsByUser :many
SELECT * FROM skills
WHERE user_id = $1 AND is_active = true
ORDER BY created_at;

-- name: UpdateSkill :one
UPDATE skills
SET proficiency      = $3,
    years_experience = $4,
    is_active        = $5,
    updated_at       = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: DeleteSkill :exec
DELETE FROM skills
WHERE id = $1 AND user_id = $2;

-- name: ListContributionsBySkill :many
SELECT contribution_id FROM v_skill_provenance
WHERE skill_id = $1;

-- name: ListActiveSkillTagNamesByUser :many
SELECT t.name
FROM skills s
JOIN tags t ON t.id = s.tag_id
WHERE s.user_id = $1 AND s.is_active = true;

-- name: ListActiveSkillMatchTermsByUser :many
-- Everything the fit-gate matcher needs to answer a JD requirement: the
-- canonical name, the synonyms a JD might use instead, and the category that
-- carries the competency vocabulary a JD phrases requirements in. Selecting
-- t.name alone is what made "Golang" a gap against a stored "Go", and
-- "CI/CD" a gap against Jenkins.
--
-- The join is constrained on user_id as well as tag_id. FK integrity on
-- skills.tag_id already implies it; stating it keeps the row set correct if a
-- tag is ever re-pointed.
SELECT t.name, t.aliases, t.category, c.aliases AS category_aliases
FROM skills s
JOIN tags t ON t.id = s.tag_id AND t.user_id = s.user_id
JOIN tag_categories c ON c.user_id = t.user_id AND c.name = t.category
WHERE s.user_id = $1 AND s.is_active = true;
