-- Reverses 014.
--
-- Restoring NOT NULL will fail if any project-only contribution exists, since
-- there is no employer to reassign it to and inventing one would be worse than
-- refusing. That is the correct behavior: delete or reassign those rows first
-- if you genuinely mean to go back.

DROP TRIGGER IF EXISTS project_contributions_leave_a_home ON project_contributions;
DROP TRIGGER IF EXISTS contributions_require_a_home ON contributions;
DROP FUNCTION IF EXISTS contribution_requires_a_home();

ALTER TABLE contributions
  ALTER COLUMN position_id SET NOT NULL;
