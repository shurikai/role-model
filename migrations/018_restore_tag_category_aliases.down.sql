-- Deliberately a no-op.
--
-- The up migration is a COALESCE backfill of a column that already existed
-- (012 added it). There is no schema change to reverse, and no way to reverse
-- the data change correctly: after it runs, a populated aliases row is
-- indistinguishable from one a user curated by hand. Setting the six
-- categories back to NULL would delete that curation, and NULL is not a state
-- worth restoring in the first place — it is the bug this migration fixes.
--
-- Rolling back past 018 leaves the vocabulary in place, which is the safe
-- direction. Dropping the column entirely is 012's down migration's job.

SELECT 1;
