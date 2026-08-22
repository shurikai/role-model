-- Rename the signal vocabulary that only made sense for one industry, and
-- delete the two closed enums that were losing information to a free-text
-- field sitting right beside them.
--
-- This is a clean break. Stored jd_signals blobs keep their old keys; nothing
-- reads them any more, and re-extraction is the intended recovery. Documents
-- generated before this migration validate against resume.v1.json and not
-- against v2 -- regenerate the ones worth keeping.

-- ============================================================
-- preferences.preference_type
-- ============================================================
-- work_type -> role_shape. The name meant two different things in two places
-- and the collision cost 23 weight of guaranteed gap on every evaluation
-- (#53): jd_signals.work_type was remote|hybrid|onsite|unknown -- work
-- ARRANGEMENT -- while preference_type 'work_type' meant kind-of-work
-- ("backend", "platform engineering", "small team, high ownership"). No label
-- of the second kind can ever appear in a four-value enum of the first.
--
-- anti_pattern -> dealbreaker. "Anti-pattern" is a software-design term being
-- used to mean "a thing that rules this role out", which is what the column
-- has always held.
--
-- primary_stack -> core_practice. A stack is software's word for it. The
-- distinction the type carries is not software-specific at all: it is
-- prominence versus presence, and "Spanish fluency as a core requirement"
-- needs it exactly as much as "Python as a primary language" does.
ALTER TABLE preferences DROP CONSTRAINT check_preference_type;

UPDATE preferences SET preference_type = 'role_shape'    WHERE preference_type = 'work_type';
UPDATE preferences SET preference_type = 'dealbreaker'   WHERE preference_type = 'anti_pattern';
UPDATE preferences SET preference_type = 'core_practice' WHERE preference_type = 'primary_stack';

ALTER TABLE preferences ADD CONSTRAINT check_preference_type
    CHECK (preference_type IN ('domain', 'role_shape', 'culture', 'dealbreaker', 'core_practice'));

-- ============================================================
-- fit_reports
-- ============================================================
-- technical_* -> capability_*. The fit gate scores whether the person can do
-- what the posting asks for, which is a capability question in any field. The
-- column name said the answer had to be technical, and that reading had teeth:
-- it is the same assumption that made jd_extraction.tmpl refuse to extract a
-- clinical or pedagogical requirement as a requirement at all.
ALTER TABLE fit_reports RENAME COLUMN technical_score   TO capability_score;
ALTER TABLE fit_reports RENAME COLUMN technical_gaps    TO capability_gaps;
ALTER TABLE fit_reports RENAME COLUMN technical_matches TO capability_matches;
ALTER TABLE fit_reports RENAME COLUMN technical_partial TO capability_partial;

-- anti_pattern_passed is derived from the gate-hit list rather than being the
-- only record of it, so the plural reads better: nothing was tripped.
ALTER TABLE fit_reports RENAME COLUMN anti_pattern_passed TO dealbreakers_clear;
ALTER TABLE fit_reports RENAME COLUMN anti_pattern_hits   TO dealbreaker_hits;

-- ============================================================
-- projects
-- ============================================================
-- role and status were open-source repository vocabulary. A portfolio of
-- buildings, cases, recipes, or performances has no author/maintainer and no
-- dormant/archived. Nothing branches on either value -- the renderer prints
-- them -- so they become free text rather than a second enum.
ALTER TABLE projects DROP CONSTRAINT projects_role_check;
ALTER TABLE projects DROP CONSTRAINT projects_status_check;

-- repo_url -> source_url. A repository is one kind of source; a case number, a
-- catalogue entry, or a score is another.
ALTER TABLE projects RENAME COLUMN repo_url TO source_url;
