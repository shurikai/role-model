-- Separates a preference's *severity* from its *gate behavior*.
--
-- `hard_exclude` was a sentiment value carrying no weight (enforced by
-- check_exclude_has_no_weight), which meant ScorePreferenceFit had nothing to
-- score and skipped those rows entirely. That was correct while the gate
-- blocked generation — an excluded JD was never scored, so a score
-- contribution was moot. Once the gate became advisory (every evaluation now
-- produces a complete report) the skip became a hole: the strongest signal in
-- the preference model contributed nothing to the only number that survives,
-- and an excluded role scored as though the exclusion had never been recorded.
--
-- After this migration every row is a weighted positive or negative, and
-- `is_hard_gate` additionally marks the rows that trip the gate. A hard
-- exclude is now expressible as what it actually is: a heavy negative that
-- also gates.
--
-- Step order is load-bearing. The backfill must run before either constraint
-- change, because existing hard_exclude rows hold both a NULL weight and a
-- sentiment value the narrowed CHECK will reject.

-- 1. Safe first: the default backfills every existing row.
ALTER TABLE preferences ADD COLUMN is_hard_gate BOOLEAN NOT NULL DEFAULT FALSE;

-- 2. Rewrite the hard_exclude rows before the constraints tighten around them.
--    Weight 10 is the top of the 1-10 scale; severity among gates is tunable
--    per row afterward.
UPDATE preferences
SET    sentiment    = 'negative',
       is_hard_gate = TRUE,
       weight       = 10,
       updated_at   = now()
WHERE  sentiment = 'hard_exclude';

-- 3. The weight/sentiment coupling this constraint enforced no longer exists.
ALTER TABLE preferences DROP CONSTRAINT check_exclude_has_no_weight;

-- 4. Every preference now carries a weight. Fails if step 2 did not run.
ALTER TABLE preferences ALTER COLUMN weight SET NOT NULL;

-- 5. Narrow the sentiment domain. Fails if step 2 did not run.
ALTER TABLE preferences DROP CONSTRAINT check_sentiment;
ALTER TABLE preferences ADD  CONSTRAINT check_sentiment
  CHECK (sentiment IN ('positive', 'negative'));

CREATE INDEX ON preferences(user_id, is_hard_gate);
