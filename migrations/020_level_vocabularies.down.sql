-- Restoring the CHECKs first: a row whose level or proficiency was written
-- while the vocabulary was open would make these fail, and failing before the
-- tables are dropped leaves the vocabulary intact to inspect.
ALTER TABLE positions ADD CONSTRAINT positions_industry_level_check
    CHECK (industry_level IN (
        'junior','mid','senior','staff','principal',
        'lead','manager','director','vp','ic'
    ));

ALTER TABLE skills ADD CONSTRAINT check_proficiency
    CHECK (proficiency IN ('novice', 'proficient', 'expert'));

DROP TABLE proficiency_levels;
DROP TABLE career_levels;
