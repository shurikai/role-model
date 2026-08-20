-- Reverses 015. Rows move back to anti_pattern and regain their original
-- labels before the CHECK narrows, or the narrowed CHECK would reject them.

BEGIN;

UPDATE preferences
SET    preference_type = 'anti_pattern',
       label           = 'expert Python as primary requirement',
       updated_at      = now()
WHERE  preference_type = 'primary_stack'
  AND  label           = 'Python as a primary language';

UPDATE preferences
SET    preference_type = 'anti_pattern',
       label           = 'production LLM / AI engineering as hard requirement',
       updated_at      = now()
WHERE  preference_type = 'primary_stack'
  AND  label           = 'LLM / AI engineering as the primary job';

UPDATE preferences
SET    preference_type = 'anti_pattern',
       updated_at      = now()
WHERE  preference_type = 'primary_stack';

UPDATE preferences
SET    label      = 'product over platform / internal tooling',
       updated_at = now()
WHERE  preference_type = 'work_type' AND label = 'product engineering';

UPDATE preferences
SET    label      = 'platform / internal tooling over product',
       updated_at = now()
WHERE  preference_type = 'work_type' AND label = 'internal platform / developer tooling';

UPDATE preferences
SET    label      = 'greenfield over pure maintenance',
       updated_at = now()
WHERE  preference_type = 'work_type' AND label = 'greenfield development';

ALTER TABLE preferences DROP CONSTRAINT check_preference_type;
ALTER TABLE preferences ADD  CONSTRAINT check_preference_type
  CHECK (preference_type IN ('domain', 'work_type', 'culture', 'anti_pattern'));

COMMIT;
