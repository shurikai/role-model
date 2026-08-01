-- Plain-language screening facts extracted in Stage 1, persisted on every fit
-- report. Nullable: reports written before this migration have none, and a
-- JD whose extraction predates the screening_summary prompt field will not
-- produce one.
ALTER TABLE fit_reports ADD COLUMN screening_summary JSONB;
