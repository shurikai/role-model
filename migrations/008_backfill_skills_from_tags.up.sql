-- Backfills the `skills` table from the existing, mature tag corpus.
--
-- Context: the fit-gate scorer (internal/fitgate) reads skill names
-- exclusively from `skills`, which only gains rows via the explicit
-- CreateSkill path. Nothing has synced it from `contribution_tags` /
-- `education_tags`, so it has been a small hand-curated subset while the
-- real skill signal lives on tagged contributions. This backfill closes
-- that gap by creating a `skills` row for every tag a user actually has
-- attached to a contribution or education entry, if one doesn't exist yet.
--
-- Default proficiency is 'proficient' rather than 'expert' or 'novice':
-- these are confirmed, real-experience tags (not aspirational), but the
-- migration has no basis to distinguish depth per tag, so it deliberately
-- avoids overclaiming. Reweight individual skills afterward as needed.
--
-- NOTE: this treats every used tag as skill-worthy, regardless of
-- tag_categories. If any category represents something that isn't really
-- a "skill" (e.g. a domain or outcome-type category rather than a
-- language/tool/platform), review afterward and deactivate or delete
-- those specific rows — the ON CONFLICT DO NOTHING below makes it safe to
-- rerun after manual cleanup.

INSERT INTO skills (id, user_id, tag_id, proficiency, years_experience, is_active)
SELECT
    gen_random_uuid(),
    t.user_id,
    t.id,
    'proficient',
    NULL,
    TRUE
FROM tags t
WHERE EXISTS (
    SELECT 1 FROM contribution_tags ct
    JOIN contributions c ON c.id = ct.contribution_id
    WHERE ct.tag_id = t.id AND c.user_id = t.user_id AND c.is_active = TRUE
)
OR EXISTS (
    SELECT 1 FROM education_tags et
    JOIN education e ON e.id = et.education_id
    WHERE et.tag_id = t.id AND e.user_id = t.user_id
)
ON CONFLICT (user_id, tag_id) DO NOTHING;
