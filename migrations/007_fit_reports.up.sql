CREATE TABLE fit_reports (
    id                  UUID          PRIMARY KEY,
    user_id             UUID          NOT NULL REFERENCES users(id),
    application_id      UUID          REFERENCES applications(id),
    anti_pattern_passed BOOLEAN       NOT NULL,
    anti_pattern_hits   JSONB,
    technical_score     NUMERIC(5,2),
    technical_gaps      JSONB,
    preference_score    NUMERIC(5,2),
    preference_gaps     JSONB,
    narrative           TEXT,
    created_at          TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX ON fit_reports(user_id);
CREATE INDEX ON fit_reports(application_id);
