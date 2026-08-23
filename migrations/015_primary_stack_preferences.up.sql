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

-- ---------------------------------------------------------------------------
-- The row rewrites that used to follow have moved to the seed files.
--
-- They rewrote six of one person's private preference rows by quoting their
-- labels verbatim, permanently, in the applied migration history of a public
-- repository. Nobody cloning this repo has those preferences, and the schema
-- change above is the only part of this migration that is about the schema.
--
-- Migrations own structure; seeds own vocabulary, and a preference label is
-- vocabulary in the most literal sense: it is the words someone chose for what
-- they want. The retyping these statements performed is carried by
-- 021_preference_reconciliation.sql in the private seed and by
-- database/sample/007_skills_preferences.sql, both of which create the rows
-- with the right type in the first place.
--
-- Stripping them changes nothing for a database that already ran them.
-- ---------------------------------------------------------------------------

COMMIT;
