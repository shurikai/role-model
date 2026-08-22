ALTER TABLE projects RENAME COLUMN source_url TO repo_url;

ALTER TABLE projects ADD CONSTRAINT projects_role_check
    CHECK (role IN ('author', 'maintainer', 'contributor', 'lead'));
ALTER TABLE projects ADD CONSTRAINT projects_status_check
    CHECK (status IN ('active', 'dormant', 'archived'));

ALTER TABLE fit_reports RENAME COLUMN dealbreaker_hits   TO anti_pattern_hits;
ALTER TABLE fit_reports RENAME COLUMN dealbreakers_clear TO anti_pattern_passed;

ALTER TABLE fit_reports RENAME COLUMN capability_partial TO technical_partial;
ALTER TABLE fit_reports RENAME COLUMN capability_matches TO technical_matches;
ALTER TABLE fit_reports RENAME COLUMN capability_gaps    TO technical_gaps;
ALTER TABLE fit_reports RENAME COLUMN capability_score   TO technical_score;

ALTER TABLE preferences DROP CONSTRAINT check_preference_type;

UPDATE preferences SET preference_type = 'work_type'     WHERE preference_type = 'role_shape';
UPDATE preferences SET preference_type = 'anti_pattern'  WHERE preference_type = 'dealbreaker';
UPDATE preferences SET preference_type = 'primary_stack' WHERE preference_type = 'core_practice';

ALTER TABLE preferences ADD CONSTRAINT check_preference_type
    CHECK (preference_type IN ('domain', 'work_type', 'culture', 'anti_pattern', 'primary_stack'));
