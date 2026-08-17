# Role Model — Data Model Reference

**Generated:** August 12, 2026
**Sources:** `migrations/001`–`010` (authoritative), `internal/db/` (sqlc models
and queries), cross-checked against the running dev database on port 5433.
**Schema version:** migration 10, clean (`schema_migrations.dirty = false`)

This is a *current-state reference*. It is not the same document as
`notes/role-model-schema-design.md`, which is the June 2026 **design** doc that
proposed the `skills` and `preferences` tables before they were built. That
document is design history and describes 18 tables; this one describes what is
actually deployed.

**22 application tables + 1 view.** (`schema_migrations` is golang-migrate's own
bookkeeping table and is excluded from that count, which is how the ROADMAP
counts it too.)

---

## Cluster map

| Cluster | Tables | What it answers |
|---|---|---|
| 1. Identity | `users` | Who owns this data, and the resume header block |
| 2. Career history core | `employers`, `positions`, `contributions` | The narrative spine — where you worked and what you did |
| 3. Supporting history | `education`, `credentials`, `projects` | Non-employment history |
| 4. Taxonomy | `tag_categories`, `tags` + 5 join tables | The retrieval vocabulary — how anything gets found |
| 5. Fit model | `skills`, `preferences`, `v_skill_provenance` | What you can do and what you want |
| 6. Application pipeline | `applications`, `resume_versions`, `fit_reports`, `contribution_feedback` | One job application's journey |
| 7. Import (Stage 0) | `import_batches`, `contribution_drafts` | Getting raw text into cluster 2 |

Ownership flows in one direction: `users` → everything. Every tenant-scoped
table carries its own `user_id` rather than relying on a join to reach one.

---

## 1. Identity

### `users`
The tenant root and the resume header block in one table. Migration 003 added
the contact/identity columns; 004 added auth.

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | Client-generated |
| `email` | TEXT | `UNIQUE` — the login identifier |
| `password_hash` | TEXT | bcrypt. **Nullable** (added by 004 after rows existed) |
| `full_name`, `phone`, `location` | TEXT | Resume header contact block |
| `linkedin_url`, `github_url`, `site_url` | TEXT | Header links |
| `headline` | TEXT | Header tagline |
| `created_at`, `updated_at` | TIMESTAMPTZ | |

Nothing references `users` by anything but `id`. Single-tenant in practice
today; the `user_id` columns everywhere else are the multi-tenant path.

---

## 2. Career history core

This is the spine: `employers` → `positions` → `contributions`, each a
strict parent-child with `NOT NULL` FKs.

### `employers`
| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `user_id` | UUID → `users` | NOT NULL |
| `name` | TEXT | NOT NULL |
| `industry`, `notes` | TEXT | |

### `positions`
A role held at an employer. Carries **both** the verbatim title and an
industry-normalized read of it — that dual representation is deliberate, so
generation can reason about seniority without being trapped by an idiosyncratic
company title.

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `user_id` | UUID → `users` | NOT NULL |
| `employer_id` | UUID → `employers` | NOT NULL |
| `title` | TEXT | NOT NULL — verbatim company title |
| `industry_level` | TEXT | CHECK: `junior`, `mid`, `senior`, `staff`, `principal`, `lead`, `manager`, `director`, `vp`, `ic` |
| `industry_role` | TEXT | Normalized role, free text |
| `level_rationale` | TEXT | *Why* the normalized level was assigned |
| `started_on` | DATE | NOT NULL |
| `ended_on` | DATE | NULL = current |
| `location` | TEXT | Added by migration 002 |
| `context_narrative` | TEXT | Situational framing for the LLM |
| `sort_order` | INT | Default 0 — manual ordering override |

### `contributions`
The atomic unit of retrieval, and richer than a resume bullet on purpose. The
three descriptive fields are kept separate so generation gets distinct signals
rather than one undifferentiated blob.

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `user_id` | UUID → `users` | NOT NULL |
| `position_id` | UUID → `positions` | NOT NULL |
| `summary` | TEXT | NOT NULL — short form |
| `full_description` | TEXT | NOT NULL — the detail |
| `outcomes` | TEXT | Results/impact |
| `scale_context` | TEXT | Size/scope framing |
| `is_active` | BOOL | Default TRUE. **Soft delete** — FALSE = invisible to generation, still queryable |

---

## 3. Supporting history

Three independent tables hanging directly off `users` — none of them nest under
`employers` or `positions`.

### `education`
`institution` (NOT NULL), `degree`, `field_of_study`, `started_on`, `ended_on`,
`notes`.

### `credentials`
`name` (NOT NULL), `issuer`, `issued_on`, `expires_on`, `credential_url`.

### `projects`
Parallel to positions but not employer-scoped. Carries explicit generation
overrides that no other table has.

| Column | Type | Notes |
|---|---|---|
| `name` | TEXT | NOT NULL |
| `tagline` | TEXT | |
| `role` | TEXT | CHECK: `author`, `maintainer`, `contributor`, `lead`. Default `author` |
| `status` | TEXT | CHECK: `active`, `dormant`, `archived`. Default `active` |
| `started_on`, `ended_on` | DATE | |
| `repo_url`, `live_url`, `writeup_url` | TEXT | |
| `force_include` | BOOL | Default FALSE — bypass relevance selection |
| `force_exclude` | BOOL | Default FALSE — never select |

---

## 4. Taxonomy

Tags are the retrieval mechanism. Without a tag, a contribution is effectively
invisible to JD-driven selection.

### `tag_categories`
`name`, `sort_order`, and `UNIQUE (user_id, name)`. That composite unique is
load-bearing — see below.

### `tags`
| Column | Type | Notes |
|---|---|---|
| `name` | TEXT | `UNIQUE (user_id, name)` |
| `aliases` | TEXT[] | Native Postgres array — normalization for JD term matching |
| `category` | TEXT | NOT NULL |
| `sort_order` | INT | Default 0 |

The interesting bit: `tags` references `tag_categories` by a **composite foreign
key on `(user_id, category)` → `(user_id, name)`**, not by a category UUID. So a
tag's category is stored as its literal name, and the FK guarantees you can't
assign a category belonging to another user. Renaming a category is therefore a
schema-level operation, not a single-row update.

### Join tables
All five are pure two-column composite-PK joins with no payload columns:

| Table | Links |
|---|---|
| `contribution_tags` | `contributions` ↔ `tags` |
| `education_tags` | `education` ↔ `tags` |
| `credential_tags` | `credentials` ↔ `tags` |
| `project_tags` | `projects` ↔ `tags` |
| `project_contributions` | `projects` ↔ `contributions` |

`project_contributions` is the odd one out — it joins two first-class entities
rather than an entity to a tag. It is currently **empty** (tracked as issue #22).

---

## 5. Fit model

Added by migration 005, backfilled by 008. This cluster is what `internal/fitgate`
reads.

### `skills`
A per-user depth annotation on a tag. Note that a skill *is* a tag — the table
adds proficiency and duration on top rather than introducing a second vocabulary.

| Column | Type | Notes |
|---|---|---|
| `user_id` | UUID → `users` | NOT NULL |
| `tag_id` | UUID → `tags` | NOT NULL. `UNIQUE (user_id, tag_id)` |
| `proficiency` | TEXT | NOT NULL. CHECK: `novice`, `proficient`, `expert` |
| `years_experience` | NUMERIC(4,1) | Nullable |
| `is_active` | BOOLEAN | Default TRUE |

### `v_skill_provenance` (VIEW)
The only view in the schema. Derives skill → contribution links automatically by
joining `skills` to `contribution_tags` on `tag_id`, so evidence for a skill
claim never has to be maintained by hand:

```sql
SELECT s.id AS skill_id, s.user_id, ct.contribution_id
FROM skills s
JOIN contribution_tags ct ON ct.tag_id = s.tag_id;
```

Was designed as a table (`skill_provenance`), shipped as a view — correctly, since
it holds no information the join doesn't already have.

### `preferences`
What you want and don't want in a role. Read by `ScorePreferenceFit` and
`RunAntiPatternGate`.

| Column | Type | Notes |
|---|---|---|
| `preference_type` | TEXT | CHECK: `domain`, `work_type`, `culture`, `anti_pattern` |
| `label` | TEXT | NOT NULL. `UNIQUE (user_id, preference_type, label)` |
| `sentiment` | TEXT | CHECK: `positive`, `negative` |
| `weight` | SMALLINT | CHECK 1–10, NOT NULL |
| `is_hard_gate` | BOOLEAN | NOT NULL DEFAULT FALSE |
| `context_type` | TEXT | CHECK: `permanent`, `contract`, `fractional`, or NULL |
| `notes` | TEXT | |

Severity and gate behavior are separate columns (migration 011). `weight` says
how much a preference matters; `is_hard_gate` says whether matching it
disqualifies. A hard exclude is a heavy negative that also gates.

This replaces a `hard_exclude` sentiment whose rows were required to carry a
NULL weight, on the reasoning that a categorical exclusion has no meaningful
weight. That held while the gate blocked generation — an excluded JD was never
scored, so there was nothing for a weight to feed. Once the gate became
advisory and every JD got scored regardless, a weightless exclusion meant the
strongest preference signal contributed nothing to the score, and an excluded
role read as a good fit. Weight is what the scorer needs to say otherwise.

Note that `weight` alone does not make an exclusion decisive: `internal/fitgate`
keeps gate rows out of the normalized average and caps the score on a match.
Under a normalized average the denominator grows with the penalty, so no single
row can dominate on weight alone.

`preference_type` is doing double duty: it names both the *subject* of the
preference and (via `anti_pattern`) a kind of sentiment. That overlap is what the
`gateFieldsFor` routing in `internal/fitgate/scorer.go` has to work around, and
it's documented at length in `notes/hard-exclude-preference-audit.md`.

---

## 6. Application pipeline

One row in `applications` is one job application; everything else in this cluster
hangs off it.

### `applications`
| Column | Type | Notes |
|---|---|---|
| `company_name`, `role_title` | TEXT | NOT NULL |
| `jd_url`, `jd_text` | TEXT | Raw job description |
| `jd_signals` | JSONB | **Stage 1 output** — extracted structured signals |
| `status` | TEXT | CHECK: `draft`, `applied`, `screen`, `interview`, `offer`, `rejected`, `withdrawn` |
| `applied_on` | DATE | Parsing on update is broken — issue #6 |
| `notes` | TEXT | |

### `resume_versions`
| Column | Type | Notes |
|---|---|---|
| `application_id` | UUID → `applications` | NOT NULL. `UNIQUE (application_id, version_number)` |
| `version_number` | INT | NOT NULL |
| `generation_params` | JSONB | Per-call prompt blob hashes + `pipelineVersion` — the traceability record |
| `structured_output` | JSONB | **NOT NULL** — the resume document, validated against `schema/resume.v1.json` |
| `generation_notes` | TEXT | |
| `submitted` | BOOL | Default FALSE |

Rendered `.docx` bytes are **not** stored here — they're returned to the caller.
Blob storage is issue #10.

### `fit_reports`
Written by `internal/fitgate` before generation. Grew three columns after its
initial migration, each fixing a distinct narrative defect.

| Column | Type | Added |
|---|---|---|
| `application_id` | UUID → `applications` | 007. **Nullable** — the only nullable parent FK in the schema |
| `anti_pattern_passed` | BOOLEAN | 007. NOT NULL. Now *informational* — no longer blocks generation |
| `anti_pattern_hits` | JSONB | 007 |
| `technical_score` | NUMERIC(5,2) | 007 |
| `technical_gaps` | JSONB | 007 |
| `preference_score` | NUMERIC(5,2) | 007 |
| `preference_gaps` | JSONB | 007 — positive preferences the JD didn't mention |
| `narrative` | TEXT | 007 — LLM prose, written *from* the scores |
| `preference_conflicts` | JSONB | **009** — negative preferences the JD actively matched |
| `screening_summary` | JSONB | **010** — plain-language screening facts from Stage 1 |

`preference_gaps` and `preference_conflicts` must stay separate. Collapsing them
made the narrative describe merely-unmentioned preferences as active conflicts.

### `contribution_feedback`
Scoped to a `(contribution, resume_version)` pair — feedback is per-version, not
global to the contribution.

`accepted_bullets` / `rejected_bullets` are `TEXT[]`; `edited_deltas` is JSONB.
Currently **empty** — the feedback loop endpoint is issue #9.

---

## 7. Import (Stage 0)

Migration 006. A staging area so LLM-extracted content is reviewed by a human
before it reaches `contributions`.

### `import_batches`
`raw_text` (NOT NULL), plus `status` with CHECK `pending`, `extracting`,
`enriching`, `review`, `complete`, `failed`, and `error_text`.

### `contribution_drafts`
Mirrors the shape of `contributions` (`summary`, `full_description`, `outcomes`,
`scale_context`) but stores `employer_name` and `position_title` as **plain text**
rather than FKs — the draft may reference an employer that doesn't exist yet.
Resolving those strings to real FK rows is what approval does.

Adds `flags` (JSONB, for flagged inferences) and `status` with CHECK `pending`,
`approved`, `rejected`.

Note: these two tables differ from the ROADMAP's proposed DDL — the shipped
version uses `batch_id` (not `import_batch_id`), `status` (not `review_status`),
and drops `raw_text`/`suggested_text`/`confidence`/`contribution_id` in favor of
the contribution-shaped fields above.

---

## Conventions

- **UUID PKs everywhere**, client-generated. Tables from migration 001/005 declare
  `DEFAULT gen_random_uuid()`; 006/007 tables omit the default and require the
  caller to supply it.
- **`created_at` / `updated_at`** on all entity tables. Join tables have neither.
  `resume_versions` and `fit_reports` have only `created_at` — both are immutable
  records by design.
- **`user_id` on every tenant-scoped table**, enforced at the query level rather
  than by RLS.
- **Soft delete** via `is_active` on `contributions` and `skills` only.
- **No `ON DELETE` clauses anywhere.** Every FK is restrict-by-default, so deletes
  of parents with children fail at the DB level. The API turns this into a
  `409 has_dependents` and does join-table cleanup transactionally in Go.
- **JSONB for LLM-shaped blobs**: `jd_signals`, `generation_params`,
  `structured_output`, `edited_deltas`, and the four `fit_reports` payloads.

---

## Live database reconciliation

Checked against `role_model` on port 5433, August 12, 2026. **The live schema
matches the migrations exactly** — 22 tables + 1 view, migration 10, not dirty.
No drift.

Row counts, with a caveat:

| Table | Rows | |
|---|---|---|
| `users` | 201 | **187 are `@test.local` integration-test residue** |
| `employers` | 109 | 8 belong to the real account |
| `positions` | 91 | 9 real |
| `contributions` | 106 (100 active) | 36 real |
| `tags` | 113 | 65 real |
| `tag_categories` | 65 | |
| `contribution_tags` | 107 | |
| `skills` | 57 | |
| `preferences` | 24 | |
| `education` / `credentials` / `projects` | 13 / 12 / 22 | |
| `applications` | 30 | |
| `resume_versions` | 27 | |
| `fit_reports` | 17 | |
| `v_skill_provenance` | 103 | Derived |
| `project_contributions` | **0** | Issue #22 |
| `contribution_feedback` | **0** | Issue #9 |
| `import_batches` / `contribution_drafts` | **0** | Stage 0 never run against this DB |

Integration tests create users and never clean up. The aggregate counts are
inflated by roughly 4× as a result; per-user counts are the meaningful ones.

---

## Two findings worth acting on

### 1. Skill depth data exists now — the scorer just doesn't read it

`CLAUDE.md` (Open Questions) and `README.md` both state that migration 008
backfilled every skill at a uniform `proficient` / `NULL`, so "a one-off prototype
and a decade of production use look identical to generation."

**That is no longer true of the data.** The live `skills` table:

- 7 `expert`, 36 `proficient`, 14 `novice`
- 40 of 57 rows have `years_experience` populated

A curation pass ran in seed files 016/018/019 after the backfill. The depth signal
is there.

**It is discarded before it reaches the scorer.** `ScoreTechnicalFit` in
`internal/fitgate/scorer.go:82` takes `skillNames []string` — a flat list of
names. It is fed by `ListActiveSkillTagNamesByUser`
(`internal/db/queries/skills.sql:37`), which selects `t.name` and nothing else.
Scoring is pure presence/absence: a matched required skill earns 2 points, a
matched preferred skill 1. `grep` for `proficiency` across `internal/fitgate`
returns nothing.

This reframes issue #44 ("Java depth underweighted, Python/Kubernetes
overcredited"). It reads as a data problem and is filed as one, but 25 years of
expert Java and a weekend of Kubernetes are both worth exactly 2 points today,
because the query drops the columns that would distinguish them. `ListActiveSkillsByUser`
already exists and returns full rows — the fix is plumbing, in Go, not seeding.

The stale claims in `CLAUDE.md` and `README.md` should be corrected.

### 2. The hard-pass list and the `preferences` rows disagree

Carried forward from the session 030 ROADMAP refresh, and confirmed here — the
`preferences` table has 24 rows, while the ROADMAP's Hard-Pass Filters list has
entries with no corresponding row at all (Ruby/Rails, C#/.NET, the Orlando onsite
constraint), and at least one direct contradiction (crypto/blockchain is a hard
pass in the doc, a `positive` at weight 5 in the seed).

`fitgate` can only act on rows that exist. Anything on that list without a
`preferences` row is documentation, not behavior.
