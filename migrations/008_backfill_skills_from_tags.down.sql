-- Down migration is necessarily best-effort: a plain DELETE FROM skills
-- would also destroy any hand-curated rows added before this migration
-- ran, which is wrong. This instead removes rows matching the backfill's
-- fingerprint (default proficiency, no years_experience, untouched since
-- creation). If you've edited a backfilled row since (set
-- years_experience, changed proficiency, etc.), it's correctly preserved.
-- The one gap: a *hand-added* skill that happens to match this same
-- fingerprint (proficiency = 'proficient', no years set, never edited)
-- would also be removed. Given how rows are typically added through the
-- frontend form, this is expected to be rare, but check the row count
-- before trusting a rollback in production.

DELETE FROM skills
WHERE proficiency = 'proficient'
  AND years_experience IS NULL
  AND created_at = updated_at;
