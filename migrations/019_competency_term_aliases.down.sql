-- Removes exactly the terms the up migration adds, leaving any other alias on
-- those tags untouched.
--
-- Unlike 018 this IS reversible, and for a specific reason: the terms are a
-- known, closed list, so a rollback can subtract them by name rather than
-- guessing which entries it put there. 018 backfilled a whole column where a
-- populated row is indistinguishable from one a user curated, which is what
-- made its down a no-op.
--
-- 'restful' and 'rest api' are deliberately not removed from REST — both
-- predate this migration (seed 013), and a down migration that deletes
-- vocabulary it did not add is how a rollback quietly costs you a match. The
-- up migration only adds 'rest api' where it is absent, so on a database that
-- already had it this is exactly symmetric; on one that did not, the term
-- survives the rollback. That asymmetry errs toward keeping a match.

UPDATE tags SET aliases = ARRAY(
  SELECT existing
  FROM unnest(COALESCE(aliases, '{}')) AS existing
  WHERE lower(existing) NOT IN ('backend systems', 'backend services',
                                'backend engineering', 'backend development')
)
WHERE lower(name) IN ('java', 'microservices', 'distributed systems', 'rest');

UPDATE tags SET aliases = ARRAY(
  SELECT existing
  FROM unnest(COALESCE(aliases, '{}')) AS existing
  WHERE lower(existing) NOT IN ('apis', 'restful api')
)
WHERE lower(name) = 'rest';
