CREATE TABLE skills (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id           UUID NOT NULL REFERENCES users(id),
  tag_id            UUID NOT NULL REFERENCES tags(id),
  proficiency       TEXT NOT NULL,
  years_experience  NUMERIC(4,1),
  is_active         BOOLEAN NOT NULL DEFAULT TRUE,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (user_id, tag_id),
  CONSTRAINT check_proficiency CHECK (proficiency IN ('novice', 'proficient', 'expert'))
);

CREATE VIEW v_skill_provenance AS
SELECT
    s.id AS skill_id,
    s.user_id,
    ct.contribution_id
FROM skills s
JOIN contribution_tags ct ON ct.tag_id = s.tag_id;

CREATE TABLE preferences (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id           UUID NOT NULL REFERENCES users(id),
  preference_type   TEXT NOT NULL,
  label             TEXT NOT NULL,
  sentiment         TEXT NOT NULL,
  weight            SMALLINT,
  context_type      TEXT,
  notes             TEXT,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (user_id, preference_type, label),
  CONSTRAINT check_preference_type CHECK (preference_type IN ('domain', 'work_type', 'culture', 'anti_pattern')),
  CONSTRAINT check_sentiment CHECK (sentiment IN ('positive', 'negative', 'hard_exclude')),
  CONSTRAINT check_weight_range CHECK (weight IS NULL OR (weight >= 1 AND weight <= 10)),
  CONSTRAINT check_context_type CHECK (context_type IS NULL OR context_type IN ('permanent', 'contract', 'fractional')),
  CONSTRAINT check_exclude_has_no_weight CHECK (
    (sentiment = 'hard_exclude' AND weight IS NULL) OR (sentiment != 'hard_exclude')
  )
);

CREATE INDEX ON skills(user_id);
CREATE INDEX ON skills(user_id, is_active);
CREATE INDEX ON skills(tag_id);
CREATE INDEX ON preferences(user_id);
CREATE INDEX ON preferences(preference_type);
CREATE INDEX ON preferences(sentiment);
CREATE INDEX ON preferences(user_id, sentiment);
