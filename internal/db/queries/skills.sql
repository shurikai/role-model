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

-- name: DeleteSkill :execrows
DELETE FROM skills
WHERE id = $1 AND user_id = $2;

-- name: ListSkillsWithTagsByUser :many
-- Every claimed skill with the tag behind it, for the management API.
--
-- Distinct from ListActiveSkillProfileByUser, which is the generation prompt's
-- view: this one carries the id (nothing can be edited without it) and does
-- NOT filter on is_active, because deactivating a skill is the main thing the
-- screen is for and a list that hides what you just deactivated cannot show
-- you how to put it back.
SELECT s.id, s.tag_id, t.name, t.category, s.proficiency, s.years_experience,
       s.is_active, s.created_at, s.updated_at
FROM skills s
JOIN tags t ON t.id = s.tag_id AND t.user_id = s.user_id
WHERE s.user_id = $1
ORDER BY t.category, t.name;

-- name: ListContributionsBySkill :many
SELECT contribution_id FROM v_skill_provenance
WHERE skill_id = $1;

-- name: ListActiveSkillTagNamesByUser :many
SELECT t.name
FROM skills s
JOIN tags t ON t.id = s.tag_id
WHERE s.user_id = $1 AND s.is_active = true;

-- name: ListActiveSkillProfileByUser :many
-- The claimed skills with their depth signal, for the generation prompt.
--
-- Generation previously built the resume's Skills section out of contribution
-- tags, which are vocabulary rather than claims: a tag can be attached to a
-- contribution without ever being a skill the user asserts, and JavaScript
-- reached a rendered resume that way. It also meant proficiency and
-- years_experience were dropped at the query layer, so a 25-year expert Java
-- and a 2-year novice Python arrived at the prompt indistinguishable.
--
-- Ordered category-major, then strongest first within a category, so the
-- prompt reads the depth ranking without having to derive it. NULL years sort
-- last: an unrecorded duration is not evidence of a short one.
SELECT t.name, t.category, s.proficiency, s.years_experience
FROM skills s
JOIN tags t ON t.id = s.tag_id AND t.user_id = s.user_id
WHERE s.user_id = $1 AND s.is_active = true
ORDER BY t.category, s.years_experience DESC NULLS LAST, t.name;

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
--
-- s.proficiency is selected but s.years_experience deliberately is not. The
-- scorer compares an ordinal level against a level the JD asked for, and a JD
-- states depth as a level far more often than as a number — where it does give
-- a figure ("5+ years"), extraction reads it as a level signal rather than
-- handing the scorer a number to compare. Selecting years here would put a
-- column in front of the matcher with nothing to compare it to.
SELECT t.name, t.aliases, t.category, c.aliases AS category_aliases, s.proficiency
FROM skills s
JOIN tags t ON t.id = s.tag_id AND t.user_id = s.user_id
JOIN tag_categories c ON c.user_id = t.user_id AND c.name = t.category
WHERE s.user_id = $1 AND s.is_active = true;
