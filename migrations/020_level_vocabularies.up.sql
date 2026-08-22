-- Career and proficiency levels become user-owned vocabulary.
--
-- The seniority ladder existed in six places that were kept in sync by hand:
-- the positions.industry_level CHECK below, schema/resume.v1.json,
-- docx-renderer/models.py, the enum in jd_extraction.tmpl, and two switch
-- statements in internal/generation/generate.go. The copies had already
-- diverged -- extraction never emitted 'manager', 'director', or 'vp', so
-- three DB-valid values were unreachable from a job description, and the two
-- Go switches disagreed about where 'mid' fell.
--
-- Worse than the drift: the vocabulary was software's. A chef's "sous", a
-- clinician's "attending", and a firm's "partner" all fell through to the
-- default branch, so every non-software posting silently got senior-IC length
-- and senior-IC framing regardless of what it was hiring for.
--
-- career_levels carries the ladder AND the two levers seniority drives, as
-- sibling columns on one row. That relationship is load-bearing: length says
-- how much gets written, framing says at what altitude, and a future third
-- lever belongs beside them as a third column rather than somewhere else.

CREATE TABLE career_levels (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID NOT NULL REFERENCES users(id),

    -- The stored form: what positions.industry_level and jd_signals.seniority
    -- hold, and what the extraction prompt is told to choose from.
    value            TEXT NOT NULL,
    label            TEXT NOT NULL,

    -- Ordinal. Nothing compares level strings any more; code compares rank.
    rank             INT NOT NULL,

    -- What a posting might call this level instead.
    aliases          TEXT[],

    length_budget    TEXT NOT NULL,
    framing_guidance TEXT NOT NULL,

    -- The row used when a posting's seniority matches nothing -- including
    -- the literal "unknown" that extraction emits when it cannot tell.
    --
    -- Explicit rather than derived. Falling back to the median rank reads as
    -- the neutral choice and is not: on a ten-rung ladder the median rung is
    -- staff, so every unreadable posting would inherit the ownership framing
    -- that framing_guidance spends two rules warning against reaching for.
    -- An unrecognised seniority is not evidence of a senior role.
    is_fallback      BOOLEAN NOT NULL DEFAULT FALSE,

    -- How this row reached the account. 'default' is the neutral set copied at
    -- signup, 'inferred' is proposed by import and confirmed by the user,
    -- 'user' is hand-authored.
    source           TEXT NOT NULL DEFAULT 'user',

    sort_order       INT NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (user_id, value),
    CONSTRAINT check_career_level_source
        CHECK (source IN ('default', 'inferred', 'user'))
);

CREATE UNIQUE INDEX career_levels_one_fallback_per_user
    ON career_levels (user_id) WHERE is_fallback;

-- The depth scale skills.proficiency and jd_signals.skill_levels share.
-- Three values named for software's habits today; a licence tier (RN -> NP), a
-- union classification, a CEFR band, or a belt rank is the same shape.
--
-- No length or framing columns: proficiency answers "deep enough for this
-- requirement", which is a comparison, not an instruction to the writer.
CREATE TABLE proficiency_levels (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id),
    value       TEXT NOT NULL,
    label       TEXT NOT NULL,
    rank        INT NOT NULL,
    aliases     TEXT[],
    source      TEXT NOT NULL DEFAULT 'user',
    sort_order  INT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (user_id, value),
    CONSTRAINT check_proficiency_level_source
        CHECK (source IN ('default', 'inferred', 'user'))
);

CREATE INDEX ON career_levels (user_id);
CREATE INDEX ON proficiency_levels (user_id);

-- Backfill every account that exists now with the ladder the code used to
-- hardcode, so behaviour is unchanged for them: the same seniority produces
-- the same length budget and the same framing text it did before this
-- migration ran.
--
-- This is a one-time data migration, not the vocabulary's home. New accounts
-- get the neutral set from internal/vocabulary at signup, and the sample seed
-- carries its own rows -- whoever creates a row populates its columns, which
-- is the lesson migration 018 was written to repair.
WITH budgets AS (
    SELECT
        'Target 1 page (~8-10 bullets total across ALL positions and projects combined).'    AS short,
        'Target 1-2 pages (~12-15 bullets total across ALL positions and projects combined).' AS medium,
        'Target 2 pages (~15-18 bullets total across ALL positions and projects combined).'   AS long
), framings AS (
    SELECT
        $staff$This role is pitched at staff level or above. A reader at this level is
scanning for scope and accountability, not implementation detail alone.

On the 2-3 most JD-relevant positions, open each bullet with what was owned,
decided, or changed at the system or team level, then land the supporting
evidence — the metric, the scale, the team size — in the same sentence.

  Weaker: "Rebuilt the referral intake process, cutting average wait from
    three weeks to four days."
  Stronger: "Owned referral intake across a six-site region, cutting average
    wait from three weeks to four days by rebuilding how referrals were
    routed and triaged."

Both sentences carry the same fact. The second also says what the candidate
was responsible for. This holds in every field; nothing about it is specific
to any one kind of work.

Two hard limits on this:
  - NEVER trade the evidence for the framing. A bullet that claims ownership
    and drops the number is weaker than one that only reports the number.
    Both together, or the number alone — never the claim alone.
  - NEVER manufacture scope the source material does not support. "Owned",
    "led", "set direction for" are factual claims and need backing in the
    contribution data like any other. If the data shows the work but not the
    ownership, write the work.$staff$ AS staff,
        $plain$This role is pitched at senior level or below. Lead with the concrete work
and the outcome it produced.

Where the source material genuinely supports ownership or leadership scope,
say so — but do not reach for it. A bullet claiming ownership or scope the
contribution data does not support reads as padding, and costs more
credibility than the framing gains.$plain$ AS plain
), ladder AS (
    SELECT * FROM (VALUES
        ('junior',    'Junior',      1, ARRAY['entry level', 'entry-level', 'associate'], 'short',  'plain', FALSE),
        ('mid',       'Mid-level',   2, ARRAY['mid level', 'mid-level', 'intermediate'],  'short',  'plain', FALSE),
        ('senior',    'Senior',      3, ARRAY['senior level']::TEXT[],                    'medium', 'plain', TRUE),
        ('lead',      'Lead',        4, ARRAY['tech lead', 'team lead']::TEXT[],          'long',   'staff', FALSE),
        ('staff',     'Staff',       5, ARRAY['staff level']::TEXT[],                     'long',   'staff', FALSE),
        ('principal', 'Principal',   6, ARRAY['distinguished']::TEXT[],                   'long',   'staff', FALSE),
        ('manager',   'Manager',     7, ARRAY['engineering manager']::TEXT[],             'medium', 'plain', FALSE),
        ('director',  'Director',    8, ARRAY['senior director']::TEXT[],                 'medium', 'plain', FALSE),
        ('vp',        'VP',          9, ARRAY['vice president']::TEXT[],                  'medium', 'plain', FALSE)
    ) AS t(value, label, rank, aliases, budget, framing, is_fallback)
)
INSERT INTO career_levels
    (user_id, value, label, rank, aliases, length_budget, framing_guidance, is_fallback, source, sort_order)
SELECT
    u.id,
    l.value,
    l.label,
    l.rank,
    l.aliases,
    CASE l.budget WHEN 'short' THEN b.short WHEN 'long' THEN b.long ELSE b.medium END,
    CASE l.framing WHEN 'staff' THEN f.staff ELSE f.plain END,
    l.is_fallback,
    'default',
    l.rank
FROM users u
CROSS JOIN budgets b
CROSS JOIN framings f
CROSS JOIN ladder l
ON CONFLICT (user_id, value) DO NOTHING;

INSERT INTO proficiency_levels (user_id, value, label, rank, aliases, source, sort_order)
SELECT u.id, p.value, p.label, p.rank, p.aliases, 'default', p.rank
FROM users u
CROSS JOIN (VALUES
    ('novice',     'Novice',     1, ARRAY['beginner', 'familiarity', 'exposure']),
    ('proficient', 'Proficient', 2, ARRAY['working knowledge', 'solid']),
    ('expert',     'Expert',     3, ARRAY['deep expertise', 'advanced', 'mastery'])
) AS p(value, label, rank, aliases)
ON CONFLICT (user_id, value) DO NOTHING;

-- The closed enums the tables replace.
--
-- Deliberately dropped rather than converted into composite foreign keys onto
-- the new tables. A foreign key here would trade one closed vocabulary for
-- another: a new account starts on the neutral three-band set, so the first
-- position anyone files as 'staff' would be rejected by the database. Both
-- readers already degrade gracefully on an unrecognised value -- the level
-- lookup falls back to the row flagged is_fallback, and the proficiency scale
-- ranks an unknown value below everything, so a typo reads as "no level
-- established" rather than as clearing an expert bar.
ALTER TABLE positions DROP CONSTRAINT positions_industry_level_check;
ALTER TABLE skills DROP CONSTRAINT check_proficiency;
