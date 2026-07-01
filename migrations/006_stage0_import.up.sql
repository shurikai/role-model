CREATE TABLE import_batches (
    id          UUID        PRIMARY KEY,
    user_id     UUID        NOT NULL REFERENCES users(id),
    raw_text    TEXT        NOT NULL,
    status      TEXT        NOT NULL DEFAULT 'pending'
                            CONSTRAINT import_batches_status_check
                            CHECK (status IN (
                                'pending',
                                'extracting',
                                'enriching',
                                'review',
                                'complete',
                                'failed'
                            )),
    error_text  TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE contribution_drafts (
    id               UUID        PRIMARY KEY,
    user_id          UUID        NOT NULL REFERENCES users(id),
    batch_id         UUID        NOT NULL REFERENCES import_batches(id),
    employer_name    TEXT        NOT NULL,
    position_title   TEXT        NOT NULL,
    summary          TEXT,
    full_description TEXT,
    outcomes         TEXT,
    scale_context    TEXT,
    flags            JSONB,
    status           TEXT        NOT NULL DEFAULT 'pending'
                                 CONSTRAINT contribution_drafts_status_check
                                 CHECK (status IN (
                                     'pending',
                                     'approved',
                                     'rejected'
                                 )),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ON import_batches(user_id);
CREATE INDEX ON contribution_drafts(user_id);
CREATE INDEX ON contribution_drafts(batch_id);
