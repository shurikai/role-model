-- The column comes back empty. The scores it held are gone for good: nothing
-- recomputes them, and the arithmetic that produced them was deleted with the
-- column.
ALTER TABLE fit_reports ADD COLUMN preference_score NUMERIC(5,2);
ALTER TABLE fit_reports DROP COLUMN preference_matches;
