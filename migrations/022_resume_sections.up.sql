-- The resume's section list becomes user-owned rows.
--
-- The shape of a resume was fixed in three places that had to be edited
-- together: schema/resume.v2.json required twelve top-level keys,
-- docx-renderer/renderer/docx_builder.py called five renderers in a fixed order
-- with literal headings, and the Go assembler filled exactly those blocks. A
-- clinician who wanted PUBLICATIONS, a teacher who wanted CERTIFICATIONS above
-- EXPERIENCE, or anyone who wanted "EDUCATION & TRAINING" instead of
-- "EDUCATION" had to change Go, Python, and a JSON schema to get it.
--
-- The one part that was already open is the model: `skills` is keyed by the
-- user's own tag category names. This extends that pattern up a level.
--
-- What this does NOT do is invent new content. A section names a content block
-- the pipeline already produces; ordering, heading text, and visibility are the
-- user's. A genuinely new KIND of section still needs a content source behind
-- it, and the cheapest general answer is that `projects` is already a list of
-- dated named things with a role, a status, links, and bullets -- which is what
-- a musician's PERFORMANCES and an academic's PUBLICATIONS both are.
CREATE TABLE resume_sections (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id),

    -- Which content block this section renders. Not free text: it names a
    -- block the document actually carries, and a key nothing produces would
    -- render as silence.
    key        TEXT NOT NULL,

    -- The printed heading. This is the free-text half -- "EDUCATION" or
    -- "EDUCATION & TRAINING" or "FORMATION", against the same `key`.
    heading    TEXT NOT NULL,

    sort_order INT NOT NULL,

    -- Hidden sections stay as rows so the heading and position survive being
    -- turned off and on again. Generation omits them from the document, so the
    -- renderer never learns they exist.
    hidden     BOOLEAN NOT NULL DEFAULT FALSE,

    -- Provenance, the same three values career_levels and proficiency_levels
    -- use. Deliberately NOT the "which content feeds this" pointer -- that is
    -- `key`. Two meanings under one column name is what work_type cost us.
    source     TEXT NOT NULL DEFAULT 'user',

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (user_id, key),
    CONSTRAINT check_resume_section_source
        CHECK (source IN ('default', 'inferred', 'user'))
);

CREATE INDEX ON resume_sections (user_id, sort_order);

-- Backfill every account that exists now with the order and headings the
-- renderer hardcoded, so their documents come out byte-identical.
--
-- One-time data migration, not the manifest's home: new accounts get theirs
-- from internal/vocabulary at signup, and each seed carries its own rows.
INSERT INTO resume_sections (user_id, key, heading, sort_order, source)
SELECT u.id, s.key, s.heading, s.sort_order, 'default'
FROM users u
CROSS JOIN (VALUES
    ('summary',     'SUMMARY',     1),
    ('skills',      'SKILLS',      2),
    ('experience',  'EXPERIENCE',  3),
    ('projects',    'PROJECTS',    4),
    ('education',   'EDUCATION',   5),
    ('credentials', 'CREDENTIALS', 6)
) AS s(key, heading, sort_order)
ON CONFLICT (user_id, key) DO NOTHING;
