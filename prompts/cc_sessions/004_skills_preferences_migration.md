# CC Session: Skills & Preferences Schema Migration

Context: Role Model, Go service. Read CLAUDE.md before starting. This implements
the finalized design in docs/role-model-schema-design.md, Part B (sections
7.1-7.5 and the implementation notes in section 10). That document is the
source of truth for table shape — if anything below seems to contradict it,
the doc wins; stop and flag the discrepancy rather than guessing.

## Goal

Add two new tables (`skills`, `preferences`) and one view (`v_skill_provenance`)
to the existing 18-table schema, following the exact conventions already
established by the current schema (client-generated UUIDs, user_id tenant
isolation, sqlc-generated query layer, golang-migrate migrations).

## Target structure

### Migration

Create a new golang-migrate migration (next sequential number after the
existing 4) containing:

```sql
CREATE TABLE skills (
  id                UUID PRIMARY KEY,
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
  id                UUID PRIMARY KEY,
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
```

Add indexes for: `skills(user_id)`, `skills(user_id, is_active)`, `skills(tag_id)`,
`preferences(user_id)`, `preferences(preference_type)`, `preferences(sentiment)`,
`preferences(user_id, sentiment)`.

Include a matching `down` migration that drops the view first, then both tables.

### sqlc query files

Add `skills.sql` and `preferences.sql` under the existing query directory
(match wherever `employers.sql` / `contributions.sql` currently live — follow
existing file naming and directory convention exactly, do not introduce a new
layout). At minimum:

- `skills`: Create, GetByID, ListByUser, ListActiveByUser, Update, Delete
- `preferences`: Create, GetByID, ListByUser, ListByUserAndType,
  ListHardExcludesByUser, Update, Delete
- `v_skill_provenance`: a single read query, ListContributionsBySkill (skill_id
  param, returns contribution_id rows) — this is a SELECT against the view,
  no insert/update/delete since it's not a real table

Run `sqlc generate` after adding the query files. Confirm the generated Go
types compile cleanly with `go build ./...`.

### Guarded deletes

Match the existing pattern used elsewhere in the schema: if a skill or
preference delete is attempted and something depends on it, return
`409 has_dependents` rather than deleting silently. Skills have no children
in this schema (the view is derived, not a real FK target), so a skill delete
should NOT need a guard — confirm this reasoning is correct before implementing,
since it's a deviation from the contribution/project delete pattern, not an
oversight.

### Parent-ownership checks

Every create on `skills` and `preferences` must verify `tag_id` (for skills)
belongs to a tag the user can see, and that `user_id` matches the authenticated
caller — same pattern as nested creates elsewhere (e.g. contributions under
positions).

## Constraints

- Do not introduce a native PostgreSQL ENUM type anywhere. Use TEXT with CHECK
  constraints, per the existing schema convention and the explicit decision in
  notes/role-model-schema-design.md §9.
- Do not create a real `skill_provenance` table. It is a view. Do not add
  insert/update/delete capability to it.
- Do not add `confidence`, `evidence_type`, or `primary` columns to anything —
  these were considered and explicitly deferred (see design doc §7.2). Flag it
  back to me if you think the migration needs them; don't add them unprompted.
- Do not implement the `declared_years_experience` / `computed_years_experience`
  split. This is still an open decision (design doc §9) — implement
  `years_experience` as a single nullable column as written above. I'll handle
  the split in a follow-up migration once I've decided.
- Do not add a `generation_instruction` value to `preference_type`'s CHECK
  constraint. This was considered and deferred (design doc §7.5).
- Match existing naming and style exactly — check how `is_active`,
  `created_at`/`updated_at`, and UUID generation are handled in the existing
  migrations and replicate that, don't introduce a new convention.

## Seed data

Do not write seed data in this session. Schema and generated query layer only.
Seeding `skills`/`preferences` from existing tag data is a separate, later task.

## Required outcome

- New migration applies cleanly on a fresh database AND on the currently
  seeded dev database with no data loss.
- `down` migration cleanly reverses it.
- `skills`, `preferences` tables and `v_skill_provenance` view exist exactly as
  specified above — no additional columns, no renamed columns.
- sqlc-generated Go types compile (`go build ./...` clean).
- All CHECK constraints are present and named as specified (so future
  migrations or psql introspection can reference them by name).

## Verify before finishing

Run, in order, and report output for each:

```
make migrate-up
psql $DATABASE_URL -c "\d skills"
psql $DATABASE_URL -c "\d preferences"
psql $DATABASE_URL -c "\d v_skill_provenance"
go build ./...
make migrate-down
make migrate-up
```

The final migrate-down/migrate-up round trip confirms the down migration
actually works, not just that it exists. All six commands must succeed with
no errors. Report the `\d` output for all three objects so I can confirm the
columns and constraints match this spec exactly before I review the generated
Go code.
