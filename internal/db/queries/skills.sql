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
