-- Mirror image of the up migration. The reverse-backfill must run after the
-- constraints loosen and before check_exclude_has_no_weight is restored,
-- for the same reason the forward backfill sits where it does.

-- The is_hard_gate index is not dropped explicitly; DROP COLUMN at the end
-- takes it with the column.

-- 5. Widen the sentiment domain back so 'hard_exclude' is writable again.
ALTER TABLE preferences DROP CONSTRAINT check_sentiment;
ALTER TABLE preferences ADD  CONSTRAINT check_sentiment
  CHECK (sentiment IN ('positive', 'negative', 'hard_exclude'));

-- 4. Weight must be nullable again before the gate rows are reverted to NULL.
ALTER TABLE preferences ALTER COLUMN weight DROP NOT NULL;

-- 2. Reverse the backfill. Any severity tuning applied to gate rows since the
--    up migration is lost here — hard_exclude cannot represent a weight.
UPDATE preferences
SET    sentiment  = 'hard_exclude',
       weight     = NULL,
       updated_at = now()
WHERE  is_hard_gate;

-- 3. Restore the weight/sentiment coupling now that the rows satisfy it.
ALTER TABLE preferences ADD CONSTRAINT check_exclude_has_no_weight
  CHECK (
    (sentiment = 'hard_exclude' AND weight IS NULL) OR (sentiment != 'hard_exclude')
  );

-- 1. Last: the column every step above depended on.
ALTER TABLE preferences DROP COLUMN is_hard_gate;
