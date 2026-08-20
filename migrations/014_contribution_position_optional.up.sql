-- A contribution can belong to a personal project instead of a job.
--
-- contributions.position_id was NOT NULL, so every unit of work had to have
-- happened at an employer. Personal projects therefore had nothing to hang a
-- contribution on, project_contributions stayed empty, and assembleProjects
-- dropped all five projects before the generation prompt ever saw them —
-- including the one that is the strongest current signal for the roles being
-- targeted. Projects could not appear on a generated resume at all.
--
-- project_contributions was built to link *existing job* contributions to a
-- public project (work done at an employer that is also open source). That
-- remains valid and is why a contribution may carry both: position_id names
-- where it was done, the project link names what it was part of, and the two
-- are independent facts.
--
-- What a contribution may not be is homeless. A row with neither a position
-- nor a project link is invisible to both assemblers — it can never reach a
-- resume, and nothing would report it as unreachable.

ALTER TABLE contributions
  ALTER COLUMN position_id DROP NOT NULL;

-- Enforced by trigger rather than CHECK: a CHECK constraint cannot contain a
-- subquery, and the invariant spans two tables.
CREATE OR REPLACE FUNCTION contribution_requires_a_home()
RETURNS trigger AS $$
DECLARE
  target_id uuid;
BEGIN
  -- Branch with IF, not a CASE expression. PL/pgSQL resolves the field
  -- references in every branch of a CASE regardless of which one is taken, so
  -- `CASE ... ELSE OLD.contribution_id END` fails with "record OLD has no
  -- field contribution_id" when the trigger fires on contributions.
  IF TG_TABLE_NAME = 'contributions' THEN
    target_id := NEW.id;
  ELSE
    target_id := OLD.contribution_id;
  END IF;

  -- The contribution may itself have been deleted in this transaction, in
  -- which case there is no invariant left to violate.
  IF NOT EXISTS (SELECT 1 FROM contributions WHERE id = target_id) THEN
    RETURN NULL;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM contributions c
    WHERE c.id = target_id AND c.position_id IS NOT NULL
  ) AND NOT EXISTS (
    SELECT 1 FROM project_contributions pc
    WHERE pc.contribution_id = target_id
  ) THEN
    RAISE EXCEPTION
      'contribution % has neither a position nor a project', target_id
      USING ERRCODE = 'check_violation';
  END IF;

  RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- DEFERRABLE INITIALLY DEFERRED so a transaction can insert the contribution
-- and its project link in either order. Checking at statement time would make
-- the natural insert order illegal.
CREATE CONSTRAINT TRIGGER contributions_require_a_home
  AFTER INSERT OR UPDATE OF position_id ON contributions
  DEFERRABLE INITIALLY DEFERRED
  FOR EACH ROW EXECUTE FUNCTION contribution_requires_a_home();

-- The other direction. Without this, unlinking a project-only contribution
-- would orphan it silently, and an invariant that holds only on insert is not
-- an invariant.
CREATE CONSTRAINT TRIGGER project_contributions_leave_a_home
  AFTER DELETE ON project_contributions
  DEFERRABLE INITIALLY DEFERRED
  FOR EACH ROW EXECUTE FUNCTION contribution_requires_a_home();
