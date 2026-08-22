-- Reverses 015. Rows move back to anti_pattern and regain their original
-- labels before the CHECK narrows, or the narrowed CHECK would reject them.

BEGIN;

-- ---------------------------------------------------------------------------
-- The row rewrites that used to follow have moved to the seed files; see the
-- note in the .up.sql. Reversing the CHECK is the whole of this migration's
-- structural half, and the rows are the seeds' to state.
--
-- A database rolled back to before 015 while holding core_practice rows will
-- fail the restored CHECK. That is correct: the constraint is telling you the
-- data is ahead of the schema, and the seed is where to fix it.
-- ---------------------------------------------------------------------------

COMMIT;
