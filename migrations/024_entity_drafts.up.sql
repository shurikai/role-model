-- Generalises the import staging surface from contributions to everything a
-- career is made of.
--
-- Stage 0 drafts CONTRIBUTIONS ONLY, and ApproveDraft requires a position_id
-- that already exists and belongs to the user. So the import is unusable by
-- anyone who has not already created their employers and positions by hand --
-- which is every new user, and emphatically the non-engineer this whole effort
-- is for. That gap predates the career-neutrality work and is the actual
-- blocker for the low-friction path.
--
-- contribution_drafts already had the right idea and stopped one table short:
-- it stores employer_name and position_title as PLAIN TEXT precisely because
-- the foreign key target may not exist yet. entity_drafts is that idea with the
-- entity type as a column.
CREATE TABLE entity_drafts (
    id          UUID        PRIMARY KEY,
    user_id     UUID        NOT NULL REFERENCES users(id),
    batch_id    UUID        NOT NULL REFERENCES import_batches(id),

    -- What this draft becomes. Not a CHECK: the resolver switches on it and an
    -- unknown kind is reported as an unresolvable draft, which is a better
    -- failure than a migration that has to ship before a new draft type can be
    -- proposed. The set the resolver knows is in internal/intake.
    kind        TEXT        NOT NULL,

    -- The drafted fields, shaped per kind. JSONB rather than a column per
    -- entity type because the whole point is that a draft is not yet a row:
    -- it names its parents by TEXT, carries whatever the extractor found, and
    -- is missing whatever it did not.
    payload     JSONB       NOT NULL,

    -- Drafts this one needs resolved first. A position names its employer, a
    -- contribution names its position, a skill names its tag and the tag names
    -- its category -- and none of those parents has an id until the moment it
    -- is approved. The resolver walks this graph rather than assuming an order,
    -- because the order differs per batch and a wrong guess writes an orphan.
    depends_on  UUID[]      NOT NULL DEFAULT '{}',

    -- The row this draft became, once approved. Null while pending, and how a
    -- dependent draft finds its parent's real id after the fact.
    resolved_id UUID,

    -- Review flags, same shape and purpose as contribution_drafts.flags: things
    -- a human should look at before approving. The preference-label collision
    -- check writes here.
    flags       JSONB,

    status      TEXT        NOT NULL DEFAULT 'pending'
                            CONSTRAINT entity_drafts_status_check
                            CHECK (status IN ('pending', 'approved', 'rejected')),

    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ON entity_drafts (user_id);
CREATE INDEX ON entity_drafts (batch_id);
CREATE INDEX ON entity_drafts (batch_id, status);
