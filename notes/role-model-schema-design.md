# Role Model — Database Schema Design Doc

**Status:** Built schema (confirmed) + skills/preferences design (reconciled after two-reviewer pass) — June 30, 2026

## 1. Purpose of This Document

This document has two parts. Part A records the schema that is already built, migrated, and verified in the running Role Model database — this is reference material, not up for debate. Part B was a proposed design for two tables that don't exist yet (skills and preferences); it has since been reviewed by two independent reviewers and reconciled into a final design, recorded below.

**This document is now the source of truth for implementing the migration.** Open items that remain genuinely open are called out explicitly in §9 and §11; everything else in Part B reflects a decision, not a proposal.

---

# Part A — Existing Schema (Built & Verified)

## 2. System Context

Role Model is a self-hostable, Go-based career management platform. It stores a person's full career narrative ("contributions") in PostgreSQL and uses a two-stage LLM pipeline to synthesize tailored, versioned resumes per job application. The architecture inverts the typical resume-tool pattern: the finished resume is a generated artifact, not the source of truth — the structured data is.

Retrieval is deterministic SQL against tagged, structured rows — not vector/embedding search. This is a considered tradeoff: lower semantic recall in exchange for auditability. If the wrong contribution surfaces in a generated resume, the fix is a tag correction, not a vector re-embedding.

## 3. Current Schema — 18 Tables

All tenant-scoped tables carry a `user_id` column; isolation is enforced at every query (verified under integration tests covering multi-tenant isolation). All primary keys are client-generated UUIDs, chosen for stability across reseeding.

### 3.1 Core entities

| Table | Purpose | Notes |
|---|---|---|
| `users` | Account + auth | bcrypt password hash, JWT-based session |
| `employers` | Companies worked for | Parent of positions |
| `positions` | Roles held at an employer | Parent of contributions |
| `contributions` | Individual career narrative entries | Core unit of retrieval; has `is_active` flag |
| `projects` | Personal/side projects | Parallel to positions, not employer-scoped |
| `education` | Degrees and academic history | |
| `credentials` | Certifications | Currently 0 active rows |

### 3.2 Tagging system

Tags are the current mechanism for retrieval relevance. 54 tags exist across 9 categories (languages, frameworks, databases, cloud/infra, methodology, domain, soft skills, tools, certifications-adjacent).

| Table | Purpose |
|---|---|
| `tags` | Canonical tag vocabulary, categorized |
| `contribution_tags` | Join: contributions ↔ tags |
| `position_tags` | Join: positions ↔ tags |
| `project_tags` | Join: projects ↔ tags |
| `education_tags` | Join: education ↔ tags |

### 3.3 Generation pipeline

| Table | Purpose | Notes |
|---|---|---|
| `applications` | A job application being worked | Anchors a generation run |
| `jd_signals` | Stage 1 output: extracted JD signal | JSONB |
| `resume_versions` | Stage 2 output: a generated resume | Versioned per application |
| `generation_params` | Params used for a given generation call | JSONB |
| `structured_output` | Raw structured LLM output pre-render | JSONB, validated against `schema/resume.v1.json` |
| `contribution_feedback` | Per-contribution feedback on a resume_version | See §4 — `feedback_type` enum still open |

### 3.4 Key conventions baked into the existing schema

- Guarded deletes: deleting a resource with children returns `409 has_dependents` rather than cascading silently.
- Transactional join-table cleanup on contribution and project deletes.
- Parent-ownership is verified on every nested create (no orphaned children across tenants).
- `is_active` flag on contributions: `FALSE` means invisible to generation but still queryable (used for Pelotech and a few other low-signal entries).
- JSONB columns use sqlc column-level overrides with `pointer: true` for nullable columns — the `db_type` override alone was insufficient in sqlc 1.31.1.

## 4. Known Gap: Feedback Loop Schema (partially designed, not yet built)

A prior design session established the conceptual shape of the feedback loop but left one schema decision explicitly open. Recording it here since it's adjacent to the preferences proposal in Part B.

- Two feedback levels: whole-resume (tone, length, overall fit) and per-contribution (wrong phrasing, overstated impact, wrong contribution selected for the role).
- Feedback splits into two types with different consequences: **correction feedback** (something is wrong at the source — must propagate upstream to the canonical Obsidian doc) vs. **selection feedback** (the contribution is accurate but wrong for this particular role — no upstream change needed).
- Open item: add a `feedback_type` enum (`correction` | `selection` | `phrasing`) to `contribution_feedback` so propagation status is queryable rather than inferred from free text.
- Open item: where accumulated prompt-steering guidance lives. This is guidance that should influence future generation calls (e.g. "always lead with quantified impact") — it's a third feedback surface, not naturally a row in `contribution_feedback`. Likely needs its own table; deferred until the feedback UI is actively being built.

---

# Part B — Skills & Preferences Schema (Final Design)

*This design went through two independent reviews. Where the reviews agreed, their recommendation was adopted outright. Where they disagreed, a reconciliation decision was made and is documented inline — including the reasoning, so the choice doesn't get re-litigated without cause later.*

## 5. Problem Statement

Two things Role Model currently has no first-class representation for:

- **Skills** — these are emergent only from tag aggregation across contributions. There's no row that says "Jason has expert-level Java, 25 years" with a proficiency level or years-of-experience figure. A skill only exists as the side effect of contributions being tagged with it.
- **Preferences** — domain interests, work-type preferences, culture fit, and hard anti-patterns (e.g. "no Big Four consulting," "defense/aerospace avoided") currently live only in conversational memory across chat sessions. Nothing in the database encodes them, and nothing in the pipeline can query them.

This matters because the next roadmap item — a preference-fit scoring pipeline — needs both to be queryable by SQL, the same way contribution relevance already is. Without this, fit scoring can't follow the same auditable-retrieval pattern as the rest of the system; it would have to either re-derive preferences from prose each run (fragile, non-deterministic) or hardcode them (defeats the point of a database-backed system).

## 6. Design Goals

- Same auditability standard as the rest of the schema: a fit score's inputs should be traceable to specific rows, not opaque LLM judgment.
- Skills must reference the single canonical vocabulary (the existing `tags` table) rather than introducing a second, parallel definition of "Java" or "Kubernetes" that can drift out of sync.
- Preferences need a sentiment axis (positive draw vs. hard exclude) because "likes IoT work" and "will not work defense/aerospace" need to be scored very differently — one is a tiebreaker, the other is a gate.
- Should not require schema changes again when the preference-fit pipeline (roadmap item 2) is built — get the shape right now since it's a known consumer.
- Favor mechanisms that can't drift out of sync with their source data over mechanisms that require the user to maintain two parallel records of the same fact.

## 7. Proposed Tables

### 7.1 `skills`

A first-class skill entity. Both reviewers independently flagged the original `name`/`category` columns as a problem: they create a second canonical vocabulary that duplicates and can drift from the existing `tags` table. Reconciled design links directly to `tags` instead.

```sql
CREATE TABLE skills (
  id                UUID PRIMARY KEY,
  user_id           UUID NOT NULL REFERENCES users(id),
  tag_id            UUID NOT NULL REFERENCES tags(id),  -- canonical vocabulary lives in tags, not here
  proficiency       TEXT NOT NULL,
  years_experience  NUMERIC(4,1),        -- nullable; not always meaningful (e.g. soft skills)
  is_active         BOOLEAN NOT NULL DEFAULT TRUE,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (user_id, tag_id),
  CONSTRAINT check_proficiency CHECK (proficiency IN ('novice', 'proficient', 'expert'))
);
```

**`years_experience` — declared vs. computed.** User-entered years drift out of date silently (the same class of staleness bug that produced the MySQL/DynamoDB correction in seed data). Before implementation, decide whether to split this into `declared_years_experience` (user override) and a computed value derived from `positions.start_date`/`end_date` for tag-linked roles, surfaced as "Computed: 18.4 / Override: 20" in any future UI. Flagged here as a refinement worth making at implementation time rather than deferring indefinitely — not yet finalized, see §9.

**`is_active` — kept, reframed as a generation gate, not a staleness flag.** Initial design borrowed `is_active` from `contributions` by default. On reflection this is doing different work here: a contribution goes inactive because it's low-signal; a skill goes inactive because the person has decided not to be matched against jobs requiring it (e.g. "I have 10 years of Jenkins administration experience and will not take a role centered on it"). Kept under the same name for consistency with the rest of the schema, but documented with this distinct meaning so it isn't read as redundant with a future `last_used_date` column — they answer different questions and can coexist.

### 7.2 `skill_provenance` → implemented as a view, not a table

Original proposal was a manual junction table requiring the user to separately link skills to contributions, duplicating work already done by `contribution_tags`. Since skills now reference `tag_id` directly, provenance is mechanically derivable rather than something that needs to be hand-maintained — every contribution tagged with a skill's underlying tag *is* evidence for that skill, with no separate bookkeeping required.

```sql
CREATE VIEW v_skill_provenance AS
SELECT
    s.id AS skill_id,
    s.user_id,
    ct.contribution_id
FROM skills s
JOIN contribution_tags ct ON ct.tag_id = s.tag_id;
```

**Deferred, not rejected: per-evidence confidence/primary weighting.** One review proposed expanding provenance with `confidence`, `evidence_type`, and `primary` flags (e.g. marking one contribution as the strongest evidence for a skill vs. others as incidental mentions). That's solving a real future problem, but not one the system has yet — it's the kind of complexity the rest of this schema has deliberately avoided building ahead of need. If it becomes necessary, the migration path is clean: promote the view to a real table seeded from the view's current output, then add the new columns. Not building it now.

### 7.3 `preferences`

Reconciled outcome: **keep the original three-bucket sentiment design**, rejecting the proposal to replace it with a continuous numeric score. Sentiment-as-score (e.g. -100 to +100) is more expressive in principle, but for a single-user system where the same person is the sole data-entry point, forcing a precise numeric judgment ("is Healthcare a 30 or a 35?") on every entry is a worse trade than three buckets plus a coarse weight — it increases entry friction without a corresponding gain in scoring quality, and inconsistent numbers entered on different days are a worse failure mode than coarse-but-stable buckets.

```sql
CREATE TABLE preferences (
  id                UUID PRIMARY KEY,
  user_id           UUID NOT NULL REFERENCES users(id),
  preference_type   TEXT NOT NULL,
  label             TEXT NOT NULL,   -- e.g. "IoT/telemetry", "remote-first", "Big Four consulting"
  sentiment         TEXT NOT NULL,
  weight            SMALLINT,        -- nullable; relative importance within its type, 1-10
  context_type      TEXT,            -- nullable; e.g. "permanent", "contract", "fractional"; NULL = applies globally
  notes             TEXT,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (user_id, preference_type, label),
  CONSTRAINT check_preference_type CHECK (preference_type IN ('domain', 'work_type', 'culture', 'anti_pattern')),
  CONSTRAINT check_sentiment CHECK (sentiment IN ('positive', 'negative', 'hard_exclude')),
  CONSTRAINT check_weight_range CHECK (weight IS NULL OR (weight >= 1 AND weight <= 10)),
  CONSTRAINT check_context_type CHECK (context_type IS NULL OR context_type IN ('permanent', 'contract', 'fractional')),
  -- hard exclusions are absolute gates; a weight on one would imply a contradictory "partial ban"
  CONSTRAINT check_exclude_has_no_weight CHECK (
    (sentiment = 'hard_exclude' AND weight IS NULL) OR (sentiment != 'hard_exclude')
  )
);
```

Scoring formula for the fit-scoring pipeline (roadmap item 2): `score_contribution = sentiment_sign × weight`, where `positive = 1` and `negative = -1`. `hard_exclude` rows are never scored numerically — they're consumed entirely by the anti-pattern hard gate (stage 1 of the fit-scoring pipeline) and short-circuit before scoring begins.

### 7.4 `preference_contexts` — rejected as a separate table

Both reviews agreed this would be premature as a standalone table. Reconciled into the nullable `context_type` column on `preferences` above (`CHECK` constraint scoped to known values; `NULL` means the preference applies globally). If a context dimension ever needs to be many-to-many with preferences rather than a single classifying column, that's a clean future migration — but no concrete need has appeared yet to justify it now.

### 7.5 Considered and explicitly deferred: `generation_instruction` as a `preference_type`

One review proposed solving the still-open prompt-steering accumulation gap (§4) by adding `generation_instruction` as a fifth `preference_type` value, avoiding a future standalone table. This is a reasonable idea but was not adopted in this pass: prompt-steering guidance ("always lead with quantified impact") doesn't have sentiment or weight semantics in any meaningful sense — folding it into `preferences` to avoid creating a table is the kind of premature consolidation this schema has otherwise avoided. Recorded here so the idea isn't lost, but the §4 gap remains open and undecided, to be resolved when the feedback UI is actually built.

## 8. How This Feeds the Fit-Scoring Pipeline (context for reviewer)

Roadmap item 2 describes a three-stage scoring flow: (1) a lightweight anti-pattern hard gate runs first, consuming preferences where `sentiment = hard_exclude`; (2) technical fit (via `skills` joined through `v_skill_provenance`) and preference fit (via `preferences`, using the `sentiment_sign × weight` formula from §7.3) run in parallel for anything that passes the gate; (3) a combined report surfaces both scores plus identified gaps. The LLM writes prose narrative from the scores — it does not generate the scores itself, preserving the auditable pattern used everywhere else in the system.

This is the reason the schema needs to support querying "does this skill exist at what level" and "what's the sentiment on this domain/culture attribute" independently and cheaply — both are inputs to a deterministic scoring pass, not LLM judgment calls.

## 9. Resolved Decisions and What's Still Genuinely Open

The original review questions are resolved as follows:

| Original question | Resolution |
|---|---|
| Should `skill_provenance` be a table or derived from tags? | Resolved: implemented as `v_skill_provenance`, a view joined on `skills.tag_id`. See §7.2. |
| Is the 3-bucket sentiment enum sufficient, or does scoring need a numeric scale? | Resolved: keep 3 buckets (`positive`/`negative`/`hard_exclude`) plus `weight`; numeric scoring is derived via `sentiment_sign × weight`, not stored directly. See §7.3. |
| Does `preference_contexts` deserve a table now? | Resolved: no — folded into a nullable `context_type` column on `preferences`. See §7.4. |
| TEXT-as-enum vs. native PostgreSQL ENUM? | Resolved: TEXT with explicit `CHECK` constraints on every enum-like column. Keeps migrations simple (no `ALTER TYPE ... ADD VALUE` pain) while still enforcing validity at the database level — closes the gap where a typo in Go application code (e.g. `"expert-level"` instead of `"expert"`) could previously write silently invalid data. |
| Does `is_active` on skills mean something different than on contributions? | Resolved: yes — reframed explicitly as a generation gate, not a staleness flag. Kept under the same name; meaning documented in §7.1. |

**Still genuinely open, to be decided before or during implementation:**

- **`years_experience`: declared vs. computed.** Whether to split this into a user override plus a value computed from `positions.start_date`/`end_date`, as discussed in §7.1. This wasn't part of either review's core recommendation but came up in reconciliation discussion — worth a decision before the migration is written, not after.
- **Indexing.** Both reviews flagged the same general set: `(user_id)`, `(user_id, is_active)` on skills, `(tag_id)`, `(preference_type)`, `(sentiment)`, `(user_id, sentiment)`. Final index list should be set once the actual fit-scoring queries are drafted, since query shape should drive index choice rather than the reverse.

## 10. Implementation Notes

This design is ready to drive a migration. Recommended approach: treat the design doc update (this file) and the actual schema migration as separate, sequential pieces of work rather than combining them — the migration should be a direct, literal implementation of what's written above, with no new design decisions made mid-migration. Concretely:

- A new `golang-migrate` migration adding `skills`, `v_skill_provenance`, and `preferences` with the constraints as written in §7.
- New `sqlc` query files for both tables and the view, following the existing column-level JSONB override conventions where applicable (none of these tables use JSONB, so this should be more straightforward than the generation-pipeline tables were).
- A decision on `years_experience` (declared/computed split, per §9) made before the migration is written, not patched in after.
- Seed data for `skills` and `preferences`, likely as a new numbered seed file once career history tagging is in a state to backfill from.

## 11. Out of Scope for This Doc

- Stage 0 LLM-assisted data entry schema (`import_batches`, `contribution_drafts`) — already designed in a prior session, not part of this review.
- Feedback loop `feedback_type` enum and prompt-steering accumulation table — noted in §4 as a known gap; §7.5 records and defers a proposed solution, but the gap itself remains open.
- Frontend/API surface for managing skills and preferences — schema-first; UI follows once the migration is implemented.
