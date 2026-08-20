-- Introduces the `primary_stack` preference type and moves the rows that make a
-- claim about PROMINENCE onto it.
--
-- Several seeded preferences describe how central a technology is to a role
-- rather than whether it appears at all: "expert Python as primary
-- requirement", "TypeScript / Node.js as primary language", "Angular as
-- co-equal frontend requirement". Those qualifiers had nothing to be checked
-- against. jd_signals recorded presence only, so ScorePreferenceFit matched
-- them against required_skills, where matchesSignal's field-inside-label
-- direction fired on the bare token: "Python" is a whole word inside "expert
-- Python as primary requirement".
--
-- The consequence was not subtle. A posting whose must-have qualifications read
-- "Proficiency in Java and/or Python" tripped a hard gate, capped the
-- preference score at hardGateCeiling, and was narrated as gating on expert
-- Python as a primary requirement — wording the row supplied and the JD never
-- said. Every posting naming Python as a required skill did the same (#68).
--
-- Stage 1 now extracts jd_signals.primary_stack, and prefFieldsFor routes this
-- type at it. The rows below move; the bare-label excludes ("C# / .NET",
-- "Windows / Microsoft ecosystem", "crypto / blockchain") deliberately stay on
-- anti_pattern, where presence is the whole claim.
--
-- Step order is load-bearing: the CHECK must accept the new value before any
-- row can be written with it.

BEGIN;

-- 1. Widen the type domain. Defined in 005_add_skills_preferences.
ALTER TABLE preferences DROP CONSTRAINT check_preference_type;
ALTER TABLE preferences ADD  CONSTRAINT check_preference_type
  CHECK (preference_type IN ('domain', 'work_type', 'culture', 'anti_pattern', 'primary_stack'));

-- 2. Move the prominence-claiming rows, rewording the three whose labels only
--    parsed as prominence claims because the words were in them. Under the new
--    type the qualifier is carried by the routing, so the label can say the
--    thing plainly. UNIQUE (user_id, preference_type, label) is unaffected:
--    the type changes on every row here.
UPDATE preferences
SET    preference_type = 'primary_stack',
       label           = 'Python as a primary language',
       updated_at      = now()
WHERE  preference_type = 'anti_pattern'
  AND  label           = 'expert Python as primary requirement';

UPDATE preferences
SET    preference_type = 'primary_stack',
       label           = 'LLM / AI engineering as the primary job',
       updated_at      = now()
WHERE  preference_type = 'anti_pattern'
  AND  label           = 'production LLM / AI engineering as hard requirement';

UPDATE preferences
SET    preference_type = 'primary_stack',
       updated_at      = now()
WHERE  preference_type = 'anti_pattern'
  AND  label IN (
         'TypeScript / Node.js as primary language',
         'Ruby / Rails as primary language',
         'Angular as co-equal frontend requirement',
         'Jenkins administration as primary responsibility',
         'Oracle DBA or heavy Oracle stack'
       );

-- 3. Split an oppositional pair that shares a token bag.
--
--    prefFieldsFor now routes work_type preferences at culture_signals and
--    core_competencies, because signals.WorkType is a four-value arrangement
--    enum that no work_type label could ever appear in — 23 weight of
--    guaranteed gap on every evaluation (#53). Widening the fields exposes a
--    data problem those rows were previously too unreachable to hit.
--
--    'product over platform / internal tooling' (positive 8) and
--    'platform / internal tooling over product' (negative 5) tokenize to the
--    same bag of words. matchesSignal compares token runs and cannot tell
--    "X over Y" from "Y over X", so a JD mentioning internal tooling would earn
--    the positive AND score the conflict in the same pass. Disjoint labels are
--    the fix; the ordering preference the old wording tried to express is
--    already carried by the two weights.
--
--    'greenfield over pure maintenance' goes the same way for the same reason:
--    a maintenance-heavy posting must not earn a greenfield positive on the
--    word "maintenance".
UPDATE preferences
SET    label      = 'product engineering',
       updated_at = now()
WHERE  preference_type = 'work_type'
  AND  label           = 'product over platform / internal tooling';

UPDATE preferences
SET    label      = 'internal platform / developer tooling',
       updated_at = now()
WHERE  preference_type = 'work_type'
  AND  label           = 'platform / internal tooling over product';

UPDATE preferences
SET    label      = 'greenfield development',
       updated_at = now()
WHERE  preference_type = 'work_type'
  AND  label           = 'greenfield over pure maintenance';

COMMIT;
