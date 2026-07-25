-- Adds a column to separate genuine preference conflicts (a matched
-- negative preference — something the JD actively signals that Jason
-- dislikes) from merely-unmet preferences (a positive preference the JD
-- text simply didn't mention). ScorePreferenceFit previously only tracked
-- the latter as "gaps" and silently dropped the former; the narrative
-- prompt then described unmet preferences using conflict language,
-- because it had no way to tell the two apart.

ALTER TABLE fit_reports ADD COLUMN preference_conflicts JSONB;
