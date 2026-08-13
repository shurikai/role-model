# Role Model — Architecture Checkpoint & Roadmap

**Last updated:** August 2026  
**Repo:** github.com/shurikai/role-model  
**Data repo:** private, accessed via `SEED_DIR` env var from `make seed`  
**Companion docs:** `CLAUDE.md`, `notes/discovery-design.md`,
`notes/role-model-schema-design.md`, `notes/hard-exclude-preference-audit.md`

---

## System Identity

Role Model is a self-hostable, AI-powered career management platform. Its core
thesis: a person's career history, structured as a queryable knowledge base of
contribution "atoms," is the raw material for all downstream job search
activities — resume generation, fit assessment, cover letters, interview prep.

The LLM layer synthesizes and personalizes. SQL retrieves and filters. These
roles are not interchangeable, and the architecture enforces that separation.

This is bespoke RAG: deterministic SQL retrieval driven by Stage 1 LLM signal
extraction. The resume generation pipeline is sequential, human-in-the-loop, and
deterministic by design. The LLM's role in generation is synthesis from
structured data, not autonomous decision-making.

The data-entry surface is a different story. An agent-driven onboarding
interview (see Phase 1) uses a multi-step, tool-calling agent framework to walk
users through populating their career data conversationally. Agent patterns are
appropriate here because the task is exploratory and conversational; they remain
inappropriate for the generation pipeline itself, where auditability and
determinism are the point.

**Positioning:** personal career knowledge base, not a resume generator.
The distinction matters for retention (value compounds over time), for honest
product framing, and for any future monetization argument (the structured data
is the moat, not the prompts).

---

## Stack

| Layer | Technology |
|---|---|
| Language | Go |
| Router | chi |
| Database | PostgreSQL 16 (Docker Compose, host port 5433) |
| DB access | pgx/v5 native (no ORM) |
| Query gen | sqlc |
| Migrations | golang-migrate |
| LLM | Anthropic Go SDK (claude-sonnet-*) |
| Schema validation | santhosh-tekuri/jsonschema |
| Auth | golang-jwt/jwt/v5 + golang.org/x/crypto/bcrypt |
| Config | joho/godotenv |
| Frontend | React + TypeScript + Vite, TanStack Query, Vitest |
| Renderer | Python 3.14, FastAPI + python-docx, managed with uv |

This table covers shipped stack. Planned technology commitments live in their
phase sections.

---

## Implementation Checkpoint — What Is Built and Verified

### Infrastructure
- Docker Compose managing PostgreSQL 16 on host port 5433
- golang-migrate: 10 migrations applied
- Makefile with `db-up`, `db-down`, `migrate`, `seed`, `reset` targets, plus
  `run`, `run-frontend`, `run-renderer`, and `dev` (all three processes at once)
- `make seed` reads `SEED_DIR` and applies SQL files in order
- `internal/httputil` package (resolved the handlers↔middleware circular import)

### Schema — 22 tables + 1 view
`users`, `employers`, `positions`, `contributions`, `tags`, `tag_categories`,
`contribution_tags`, `education`, `education_tags`, `credentials`,
`credential_tags`, `projects`, `project_tags`, `project_contributions`,
`applications`, `resume_versions`, `contribution_feedback`, `skills`,
`preferences`, `fit_reports`, `import_batches`, `contribution_drafts`,
and the `v_skill_provenance` view.

`jd_signals`, `generation_params`, and `structured_output` are JSONB *columns*
(on `applications` and `resume_versions`), not tables — an earlier revision of
this doc listed them as tables.

Key design decisions baked in:
- All tenant-scoped tables carry `user_id`; isolation enforced at every query
- Client-generated UUIDs throughout (stable, referenceable)
- Guarded deletes (409 `has_dependents`) for resources with children
- Transactional join-table cleanup on contribution and project deletes
- Parent-ownership verified on every nested create
- `is_active` flag on contributions: `FALSE` = invisible to generation, queryable
  when explicitly needed (Pelotech contributions use this)
- JSONB columns use column-level sqlc overrides with `pointer: true` for
  nullable columns (`db_type` override alone does not work in sqlc 1.31.1)
- Assembly skips positions/employers with no active contributions

### Seed Data — 19 files, complete canonical career history
Lives in a separate private data repo (`shurikai/role-model-seed`), checked out
in place at `database/seed` — gitignored here, so it is never tracked by this
repo. `SEED_DIR` points at it (`./database/seed`). Numbering runs 001–016 and
018–020; **there is no 017**.
```
001_foundation.sql      — users, employers, positions, 54 tags across 9 categories
002_disney.sql          — 7 contributions
003_daugherty.sql       — 5 contributions (DynamoDB Global Tables detail added)
004_pelotech.sql        — 2 contributions (inactive; TypeScript/Node surface-on-request only)
005_aemwas.sql          — 2 contributions
006_mak.sql             — 2 contributions (1 inactive)
007_dignitas.sql        — 3 contributions
008_manifold.sql        — 4 contributions (Groovy contribution activated with "built in a week" detail)
009_lockheed.sql        — 11 contributions (2 inactive)
010_projects.sql        — 4 projects (including Role Model itself)
011_education_credentials.sql — Tulane BS CS 1999; 0 active credentials
012_additions_lua_groovy_scheme.sql — Groovy/Lua/DynamoDB tags; Neovim project; Tulane notes
013_skills.sql          — 41 skills rows, plus the tags they need that 001 lacked
014_preferences.sql     — 24 preference rows (the profile tabulated below)
015_mysql.sql           — MySQL tag + its contribution_tags link (Lockheed WARSIM, L0005)
016_corrections.sql     — reactivates the Jenkins skill; backfills users.phone
018_skill_curation.sql  — closes four open items from the 2026-07-31 provenance audit
019_spring_mvc_deactivate.sql — deactivates Spring MVC (real but unplaceable in time)
020_deactivate_graphql_artifact.sql — deactivates GraphQL (not real usable depth)
```

**Pending seed tasks:** none blocking. Both items previously listed here are
closed — the DynamoDB tag and contribution text landed in `012`, as did the
Groovy tag on the active Manifold contribution.

Low-priority cleanup, tracked but not urgent:
1. `013_skills.sql:77` still carries a `TODO` to identify which contribution
   UUIDs should carry MySQL. `015_mysql.sql` already did exactly that; the
   comment is stale and should be deleted.
2. The MySQL tag row (`90…058`) is inserted by both `013` and `015`. Harmless —
   `015` uses `ON CONFLICT (id) DO UPDATE` and the values agree — but it is two
   sources of truth for one row.

### Authentication
- bcrypt password hashing
- 24-hour JWT tokens (single token per user)
- `RequireAuth` middleware; all protected routes carry real authenticated identity
- `stubUserID` fully replaced with context-based identity

### API Endpoints — Built and Verified
```
GET  /health

POST /api/v1/auth/signup
POST /api/v1/auth/login
GET  /api/v1/auth/me                  (authenticated)

GET    /api/v1/employers
POST   /api/v1/employers
GET    /api/v1/employers/:id
PATCH  /api/v1/employers/:id
DELETE /api/v1/employers/:id          (guarded)

GET    /api/v1/employers/:employerID/positions
POST   /api/v1/positions              (employer_id in body)
GET    /api/v1/positions/:id
PATCH  /api/v1/positions/:id
DELETE /api/v1/positions/:id          (guarded)

GET    /api/v1/positions/:positionID/contributions
POST   /api/v1/contributions          (position_id in body)
GET    /api/v1/contributions/:id
PATCH  /api/v1/contributions/:id
DELETE /api/v1/contributions/:id      (transactional join-table cleanup)

POST   /api/v1/applications
GET    /api/v1/applications
GET    /api/v1/applications/:id
PATCH  /api/v1/applications/:id

POST   /api/v1/applications/:id/extract-signals  — Stage 1: JD → jd_signals JSONB
POST   /api/v1/applications/:id/generate         — Stage 2 (2a body + 2b summary)
GET    /api/v1/applications/:id/versions
GET    /api/v1/resume-versions/:id
POST   /api/v1/resume-versions/:id/render        — → .docx via docx-renderer

POST   /api/v1/applications/:applicationID/fit   — fit gate scoring
GET    /api/v1/applications/:applicationID/fit
GET    /api/v1/applications/:applicationID/fit/:reportID

POST   /api/v1/import                            — Stage 0 batch create
GET    /api/v1/import/:batchID
GET    /api/v1/import/:batchID/drafts
GET    /api/v1/import/drafts/:draftID
PUT    /api/v1/import/drafts/:draftID
POST   /api/v1/import/drafts/:draftID/approve
POST   /api/v1/import/drafts/:draftID/reject

GET/POST/DELETE /api/v1/tags, /api/v1/tag-categories
POST/DELETE     /api/v1/contributions/:id/tags[/:tagId]

GET/POST        /api/v1/education, /api/v1/credentials
PATCH/DELETE    /api/v1/education/:id, /api/v1/credentials/:id
                (no GET-by-id on either — list only)

GET    /api/v1/projects
POST   /api/v1/projects
GET    /api/v1/projects/:id
PATCH  /api/v1/projects/:id
DELETE /api/v1/projects/:id
POST   /api/v1/projects/:id/contributions
DELETE /api/v1/projects/:id/contributions/:contribID
POST   /api/v1/projects/:id/tags
DELETE /api/v1/projects/:id/tags/:tagId
```

Mutations are `PATCH`, not `PUT` — partial update is the contract everywhere
except `PUT /api/v1/import/drafts/:draftID`, which still replaces the draft
wholesale. Nested creates are flat (`POST /api/v1/positions` with `employer_id`
in the body), with parent ownership verified server-side. There is no
`DELETE /api/v1/applications/:id`.

### LLM Pipeline — Built and Verified
**Stage 1 — JD Signal Extraction**
- Input: raw JD text
- Anthropic API call against `jd_extraction.tmpl`
- Output: `jd_signals` JSONB stored on `applications` row
- Verified against real job descriptions

**Stage 2 — Resume Generation**
- `AssembleContext`: composes employer/position/contribution/project/education/
  credential data via sqlc queries into the structure the prompt expects
- Renders `resume_body.tmpl`, then `resume_summary.tmpl` (2a/2b split)
- Anthropic API call
- JSON Schema validation against `schema/resume.v1.json`
  (santhosh-tekuri/jsonschema)
- Generator injects real UUIDs for provenance post-parse (prevents LLM
  fabricating position/project IDs)
- Stores `resume_version` with full provenance traceability
- Verified against real applications with real seed data

### Testing
- Integration tests using `httptest` covering multi-tenant isolation
- `createAndGetID` helper (avoids double-close on response bodies)
- CC session prompts stored in `prompts/cc_sessions/` for scaffolding
  pattern-replication work

---

## Schema Gaps

### Skills table — BUILT (migration 005, backfilled in 008)
Shipped, but in a materially different shape than proposed below. Note the
differences before working on this:

- `skills` keys to `tag_id`, not a free-text `name` + `category`. Skills are
  therefore constrained to the existing tag corpus by construction.
- `proficiency` is `NOT NULL` and checks `novice | proficient | expert`
  (the proposal said `familiar`, not `novice`).
- **`skill_provenance` is a view (`v_skill_provenance`), not a table.** It
  derives skill→contribution links from `contribution_tags`, so provenance is
  automatic and never needs writing to. The proposed junction table below was
  not built and is not needed.

**Remaining gap — depth signal, not provenance.** Migration 008 backfilled
every used tag at a uniform `'proficient'` with `years_experience = NULL`, on
the explicit reasoning that the migration had no basis to distinguish depth.
So the columns are populated but carry no differentiating information:
a single Daugherty GCP cohort prototype and a decade of AWS are both
`proficient / NULL`. JD-relevance filtering in `resume_body.tmpl` is the
current stopgap (shipped, issue #34 closed); the permanent fix is reweighting
proficiency and years per skill, which nothing does yet.

Migration 008 also flagged a review task: it treated every used tag as
skill-worthy regardless of `tag_categories`, so domain/outcome-type tags became
skills. That curation pass has since run — seed files `016`, `018`, `019`, and
`020` reactivated Jenkins and deactivated Spring MVC, GraphQL, and the rest of
the 2026-07-31 provenance audit's open items. Rerunning 008 is still safe
(`ON CONFLICT DO NOTHING`).

Original proposal, retained for contrast:
```sql
CREATE TABLE skills (
    id              UUID PRIMARY KEY,
    user_id         UUID NOT NULL REFERENCES users(id),
    name            TEXT NOT NULL,
    category        TEXT,              -- mirrors tag categories
    proficiency     TEXT CHECK (proficiency IN ('expert','proficient','familiar')),
    years_experience INT,
    notes           TEXT,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE skill_provenance (
    skill_id        UUID NOT NULL REFERENCES skills(id),
    contribution_id UUID NOT NULL REFERENCES contributions(id),
    PRIMARY KEY (skill_id, contribution_id)
);
```

### Preferences table — BUILT (migration 005)
Preferences are now queryable by the pipeline and drive the preference half of
fit-gate scoring in `internal/fitgate/scorer.go`. Matched negative preferences
surface as `preference_conflicts` (migration 009) and are kept distinct from
merely-unmet positives — see the note in Phase 2 below.

Shipped shape differs from the proposal below: `preference_type` and
`sentiment` are `TEXT` with `CHECK` constraints rather than Postgres enums,
and the proposed `preference_contexts` table was collapsed into a single
nullable `context_type` column (`permanent | contract | fractional`) plus a
1–10 `weight`. Original proposal retained for contrast:
```sql
CREATE TYPE preference_type AS ENUM
    ('domain', 'work_type', 'culture', 'anti_pattern');
CREATE TYPE preference_sentiment AS ENUM
    ('positive', 'negative', 'hard_exclude');

CREATE TABLE preferences (
    id          UUID PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id),
    pref_type   preference_type NOT NULL,
    sentiment   preference_sentiment NOT NULL,
    label       TEXT NOT NULL,      -- e.g. "distributed systems", "Big Four consulting"
    notes       TEXT
);

-- Optional: nuance for context-dependent preferences
CREATE TABLE preference_contexts (
    id              UUID PRIMARY KEY,
    preference_id   UUID NOT NULL REFERENCES preferences(id),
    context         TEXT NOT NULL,  -- e.g. "permanent", "contract"
    override_sentiment preference_sentiment NOT NULL
);
```

Known preference profile — 24 rows, transcribed from `014_preferences.sql`,
which is the source of truth. Note that several are typed differently than an
earlier revision of this doc claimed: Big Four is `culture`, not `anti_pattern`,
and `defense / aerospace` and `pure frontend` are `domain` and `work_type`
respectively. `hard_exclude` rows carry no weight.

| Type | Sentiment | Weight | Label |
|---|---|---|---|
| domain | positive | 9 | distributed systems |
| domain | positive | 8 | IoT / telemetry / real-time data |
| domain | positive | 7 | observability |
| domain | positive | 7 | consumer-facing product |
| domain | positive | 5 | fintech / blockchain |
| domain | hard_exclude | — | defense / aerospace |
| work_type | positive | 9 | small team, high ownership |
| work_type | positive | 8 | product over platform / internal tooling |
| work_type | positive | 6 | greenfield over pure maintenance |
| work_type | negative | 7 | full-stack where frontend is co-equal |
| work_type | negative | 5 | platform / internal tooling over product |
| work_type | hard_exclude | — | pure frontend |
| culture | positive | 8 | remote-first |
| culture | positive | 7 | low-ego / async |
| culture | negative | 6 | military-coded culture |
| culture | hard_exclude | — | Big Four consulting culture |
| anti_pattern | negative | 8 | Jenkins administration as primary responsibility |
| anti_pattern | negative | 6 | Oracle DBA or heavy Oracle stack |
| anti_pattern | negative | 5 | large enterprise / Big Co process-heavy |
| anti_pattern | hard_exclude | — | TypeScript / Node.js as primary language |
| anti_pattern | hard_exclude | — | expert Python as primary requirement |
| anti_pattern | hard_exclude | — | production LLM / AI engineering as hard requirement |
| anti_pattern | hard_exclude | — | Angular as co-equal frontend requirement |
| anti_pattern | hard_exclude | — | IT consulting / staff augmentation model |

Not every row can actually fire against extracted JD signals — the
skills-shaped anti-patterns in particular depend on what `gateFieldsFor` reads.
See `notes/hard-exclude-preference-audit.md`, which is current as of its two
appended resolutions.

---

## Roadmap

### Phase 1 — Usable by humans other than me
*Gate: two non-technical test users can enter career data and get a resume
without touching the terminal.*

**Backend remaining:**
- ~~Projects, education, credentials write-CRUD~~ — BUILT
- ~~Skills + preferences schema and seed~~ — BUILT (migrations 005, 008, 009;
  seeded in `013`/`014`, curated in `016`/`018`/`019`/`020`)
- ~~Pending seed tasks (MySQL tag, Groovy verification)~~ — DONE (`012`, `015`)
- Skill *depth* population — provenance itself needs no work (`v_skill_provenance`
  derives it from `contribution_tags`), but `proficiency` and `years_experience`
  are still uniform, so generation can't tell a prototype from a decade

**Stage 0 — LLM-assisted data entry pipeline** — BUILT (migration 006,
`internal/stage0`, endpoints under `/api/v1/import`). Design retained below.  
The data-entry onboarding path for non-technical users. Without this, the
only path to seeding career data is hand-written SQL, which is not generalizeable.

```
Stage 0a — Structural extraction
  Input:  user pastes resume / LinkedIn export / free-form text
  Output: JSON — list of {employer, title, dates, raw_bullets[]}
  Goal:   segment blob into career entries; no wording judgment

Stage 0b — Contribution enrichment (per bullet)
  Input:  one raw bullet + job context
  Output: {suggested_text, confidence, flagged_inferences[], missing_fields[]}
  Goal:   stronger canonical phrasing; flag inferences; identify gaps
  Rule:   NEVER invent specific numbers — flag missing metrics instead

Stage 0c — Human review UI
  Side-by-side original vs. suggested, per contribution
  Actions: approve / edit / reject / skip
  Write-through: approved/edited rows → contribution_drafts → contributions
```

Schema additions:
```sql
CREATE TABLE import_batches (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id),
    source_hint  TEXT,
    raw_input    TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

CREATE TABLE contribution_drafts (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL REFERENCES users(id),
    import_batch_id     UUID NOT NULL REFERENCES import_batches(id),
    employer_name       TEXT NOT NULL,
    position_title      TEXT NOT NULL,
    raw_text            TEXT NOT NULL,
    suggested_text      TEXT,
    confidence          TEXT CHECK (confidence IN ('high','medium','low')),
    flagged_inferences  JSONB,
    missing_fields      JSONB,
    review_status       TEXT NOT NULL DEFAULT 'pending'
                        CHECK (review_status IN
                            ('pending','approved','edited','rejected')),
    final_text          TEXT,
    contribution_id     UUID REFERENCES contributions(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    reviewed_at         TIMESTAMPTZ
);
```

Stage 0a prompt (structural pass):
```
Parse the following resume or career history text into structured JSON.
For each career entry extract: employer_name, position_title, start_date,
end_date (as written — do not normalize), and raw_bullets (verbatim, no
rewording). Return ONLY valid JSON. No preamble, no markdown fencing.
If a field is absent, use null. Do not infer dates or titles.
```

Stage 0b prompt (per-bullet enrichment):
```
You are helping a job seeker improve the phrasing of a single career contribution
for storage in a structured career knowledge base. This stored text will later be
selected and synthesized into tailored resumes.

Employer: {{employer_name}} | Title: {{position_title}}
Original: {{raw_text}}

1. Suggest a clearer, stronger phrasing using active voice, concrete scope,
   and specific outcomes WHERE EXPLICITLY STATED in the original.
2. List any numbers, metrics, or claims you inferred rather than found stated
   — these must be flagged for human verification.
3. List fields that are missing and would strengthen this contribution.
4. Rate confidence: high / medium / low.

CRITICAL: Do not invent specific numbers. If the original says "improved
performance," do not suggest "improved performance by 40%." Flag the gap.

Return ONLY valid JSON: {suggested_text, confidence, flagged_inferences[], missing_fields[]}
```

**Onboarding agent — conversational data entry** (planned; parallel to Stage 0,
not a replacement for it).

Stage 0 handles the "paste a resume or LinkedIn export" case: there is already a
document, and the work is segmenting and enriching it. The onboarding agent
handles the case Stage 0 cannot — a user with no clean document to start from,
or whose career does not decompose neatly into pre-written bullets. Both paths
write to the same tables; they differ in how the raw material is obtained.

- **Framework: LangGraph. Decided — do not substitute CrewAI or another
  framework.** The task is a single structured interview with conditional
  branching and state, not multi-agent collaboration, which matches LangGraph's
  graph-of-nodes model rather than CrewAI's crew/role abstraction. LangGraph's
  checkpointing/interrupt pattern also mirrors the Temporal Signal-based
  human-in-the-loop design already planned for Phase 3, so the mental model
  transfers between them.
- Conducts a structured interview modeled on a recruiter/career-coach
  conversation, writing contributions as the conversation progresses.
- **It calls the existing REST API endpoints.** It does not bypass the API or
  write directly to Postgres.
- Deliberate skill-acquisition project: hands-on experience with a production
  agent framework on a real feature rather than a throwaway spike. Supports the
  FDE track (see Hard-Pass Filters).
- Eval story for v1 is self-referential: run it against the existing career data,
  where the right answers are already known, and evaluate whether it asks good
  questions and writes accurate contributions.

Design principles:
1. The trust boundary is **structural honesty, not fact-checking** — the system
   enforces that every skill claim links to a supporting contribution, but does
   not verify whether the underlying claim is true.
2. The interview produces the field's tag vocabulary as a byproduct, which is
   cheaper than pre-building a vocabulary per field.
3. Sequencing: get base functionality and the frontend solid first, then
   validate against a second field (e.g. education) before designing for many
   fields speculatively.

**Frontend:**
- React + TypeScript + Vite (decided) — BUILT: auth shell plus the application
  generation flow (`Applications`, `ApplicationNew`, `ApplicationDetail`)
- Remaining: career-data views (employer/position/contribution browsing and
  editing), and the Stage 0 review UI
- Data-entry UI must be designed with Stage 0 review flow in mind from the start
  — do not design plain forms and retrofit LLM-assist later

**Auth + deploy:**
- Google OAuth (replace bcrypt/JWT stub)
- Fly.io deployment for two external test users

---

### Phase 2 — Complete end-to-end pipeline
*Gate: JD input → .docx download without leaving the UI.* **Gate met.**

1. ~~Stage 1 + Stage 2 fully wired as API endpoints with UI surfaces~~ — BUILT.
   Stage 2 was subsequently split into two calls: 2a body (bullet + skill
   selection, seniority-informed length budget) and 2b summary (scoped to the
   bullets 2a selected, so it cannot assert unsupported claims). Facts both
   calls would otherwise decide independently — the header title, notably —
   are threaded through as explicit inputs. Keep that pattern.
2. Human-in-the-loop signal review gate (UI for reviewing jd_signals before
   generation runs — this is a product requirement, not just a nice-to-have).
   **Still outstanding** — the fit gate landed, but jd_signals themselves are
   not yet reviewable before generation.
3. ~~**Go/no-go fit gate**~~ — BUILT (`internal/fitgate`, migrations 007/009,
   `POST /api/v1/applications/{id}/fit`). Deterministic scoring in Go, LLM
   writes narrative only.
   - Shipped as a `fit_reports` table, not `fit_assessments` as named here
   - Unmet positive preferences and matched negative preferences are tracked
     as **separate** lists (`preference_gaps` vs `preference_conflicts`).
     Collapsing them made the narrative describe unmentioned preferences as
     active conflicts; do not re-merge them.
   - Still open: calibrate against outcome data once enough `applications`
     rows carry real outcomes
4. **Feedback loop:**
   - Two levels: whole-resume and per-contribution
   - `feedback_type` enum: `correction` | `selection` | `phrasing`
     - `correction`: wrong at source; propagate upstream to canonical contribution
     - `selection`: right contribution, wrong role; feeds selection heuristics
     - `phrasing`: correct content, wrong wording; feeds prompt-steering table
   - Prompt-steering accumulation: separate table injected into future generation
     calls (schema TBD, defer until feedback UI is in scope)
5. ~~Renderer (docx output)~~ — BUILT. `docx-renderer/` (FastAPI +
   python-docx), one `POST /render` endpoint, called synchronously from Go via
   `internal/renderer.Client` and surfaced as
   `POST /api/v1/resume-versions/{id}/render`.
   - Layout is explicit and compact — Word heading styles were deliberately
     dropped; do not reintroduce them
   - `keep_with_next` on section-heading and role-header paragraph chains for
     widow/orphan protection; bullets intentionally left free to break
   - Rendered bytes are returned to the caller, not persisted (see blob
     storage, Phase 4)
   - In Phase 3: becomes a Temporal activity (see below)
6. **Output reviewer (three-pass)** — runs after generation, before human
   approval gate:
   - Pass 1 — Provenance: can every claim be traced to a source contribution
     in the assembled context? Untraced content = hallucination flag.
   - Pass 2 — Overstatement: does output make stronger claims than source
     data supports? Target failure mode: confident phrasing around unmeasured
     or approximate outcomes ("improved performance by 40%" when source says
     "improved performance").
   - Pass 3 — Tone/accuracy: hedged source data presented as definitive fact;
     scope inflation without source support.
   - Output: structured JSON per pass — flagged items with resume text,
     concern type, and source contribution reference. Stored as JSONB on
     `resume_versions` row.
   - Human approval UI shows resume + all three reports together; human
     adjudicates a specific flagged list, not a blind full read.
   - Manual pipeline equivalent: `/review` endpoint taking `resume_version_id`,
     running all three passes, returning combined report. No new schema needed.
   - Latency: ~15-45 seconds for three passes, absorbed by Temporal wait state.
   - Token cost: ~$0.05-0.07 per generation run — negligible at single-user volume.
7. **Eval harness — correctness verification.** Distinct from the Phase 4
   "prompt evaluation framework," which compares quality *across prompt
   versions*. This asks a different question: is the pipeline's output
   **correct** for a known input?
   - A fixture set of JDs with known-correct signal extractions, scored
     automatically. The assertion is that Stage 1 extracts the *right* signals
     from a known JD — not merely that it produced valid JSON, which is all the
     schema check proves today.
   - Extends to Stage 2: given known signals and known career data, does
     generation select the right contributions and produce accurate bullets?
   - Builds on `tests/fixtures/`, which already holds pipeline regression
     fixtures — extend that set, do not stand up a parallel one.
   - Also an FDE credibility artifact: "I built and evaluated a production
     agentic system" is the sentence FDE screens are listening for in 2026, and
     the eval harness is what makes it true rather than aspirational.

---

### MCP Server — Protocol Surface for Career Data
*Not a phase. Infrastructure that serves several of them, placed here because it
depends on Phase 2's pipeline existing and is depended on by later work.*

Exposes the career data Role Model already holds as **MCP tools**, so any MCP
client can query it directly rather than through a copy.

- **Tools, not a static resource.** The server does not serve
  `canonical-context.md`. The entire premise of SQL-as-retrieval is that queries
  are live and parameterized; `canonical-context.md` is a derived,
  third-generation copy useful for bootstrapping a chat thread and nothing more.
  It has no role in the MCP surface.
- **Implementation:** a thin layer in front of the existing Go backend — the
  same pgx/sqlc queries the pipeline already uses, exposed as tools. No second
  retrieval path, no duplicated query logic.
- **Transport:** Streamable HTTP, the current MCP spec standard as of
  2026-07-28. HTTP+SSE is deprecated; do not build against it.
- **Process boundary is an open question.** Either a separate process alongside
  the chi API — which needs its own pgx pool and forces a decision about
  importing versus duplicating the sqlc code — or a second protocol surface in
  the same binary, which shares code cleanly but changes the deployment shape.
  Decide during implementation, not now. (Open question 8.)

Candidate initial tool surface, not exhaustive:

| Tool | Purpose |
|---|---|
| `search_contributions(query, tags?, employer?, limit?)` | The core retrieval primitive, live against Postgres |
| `get_skill(skill_name)` | Status, proficiency, supporting contributions, and honest framing notes |
| `list_employment_timeline()` | The canonical never-omit timeline |
| `check_hard_excludes(jd_signals)` | The Stage 1 gate, as a callable tool |
| `get_gap_analysis(required_skills[])` | Stage 2 gap identification |

Resources — the MCP primitive for read-only addressable data — may have a role
for stable reference data such as the tag taxonomy or the anti-pattern list.
**Career facts themselves must be tools, not resources**, for the same reason
the server does not serve a markdown snapshot.

If the pipeline is eventually orchestrated by Temporal (Phase 3), MCP tools will
need to decide whether they trigger Temporal workflows or sit below that layer.
Blocked on Temporal actually being built. (Open question 9.)

---

### Phase 3 — Discovery and distributed systems
*Gate: new postings from target companies appear automatically; pipeline runs
durably end-to-end.*

**Why Kafka and Temporal, not one or the other:**

Kafka owns "things happening continuously in the world, multiple independent
consumers." The discovery worker produces events; extraction, notification, and
future filtering consumers are independent consumer groups. Genuine fan-out.

Temporal owns "this specific application's journey through a durable,
human-gated workflow." Sequential, external API calls that can fail, human
approval wait states, must be recoverable. Temporal's canonical shape.

These are complementary, not competing. Kafka gets a JD from discovery to
"signals extracted, ready for review." Temporal takes over from there.

Building the Temporal integration also serves the FDE credibility story, and
does so from a different direction than the agent work: Temporal is general
distributed-systems orchestration infrastructure, not AI-agent technology.
Shipping it demonstrates the DIS/dead-reckoning distributed-coordination
background applied to a new domain — durable workflows rather than simulation
state — rather than a second entry in the same category as LangGraph.

**`cmd/discovery` worker** (see `notes/discovery-design.md` for full design):
- Lives in this repo as a second binary; shares `go.mod`, separate process
- Config: `companies.yaml` — one entry per target company with platform +
  identifier + poll_interval
- Adapter interface: one implementation per ATS platform

```go
type Adapter interface {
    Platform() string
    Fetch(ctx context.Context, companyIdentifier string) ([]JobPosting, error)
}
```

Confirmed public, unauthenticated JSON feeds:
- **Greenhouse:** `boards-api.greenhouse.io/v1/boards/{token}/jobs?content=true`
  — confirmed for `grafanalabs`, `temporaltechnologies`
- **Ashby:** `api.ashbyhq.com/posting-api/job-board/{clientname}?includeCompensation=true`
  — includes `descriptionPlain` field; no HTML stripping needed
- **Lever:** `api.lever.co/v0/postings/{company}?mode=json` — not yet verified
  against a real target slug; verify before writing adapter
- GitHub: does not use a standalone Greenhouse/Lever board; runs under
  Microsoft's own careers system — likely no clean public API; handle as
  manual or skip

De-duplication: `discovered_postings` table in the same Postgres instance,
keyed on `(source_platform, source_company, external_id)`.

**Kafka event taxonomy:**

| Topic | Producer | Consumers |
|---|---|---|
| `job_postings.discovered` | discovery worker | extraction consumer, notification consumer |
| `jd_signals.extracted` | extraction consumer | notification consumer, (future) filtering |

**Temporal workflow — generation pipeline:**
```
Workflow: GenerateResume(applicationID)
  Activity: ExtractSignals(jdText) → signals
  Signal:   HumanApproval(approved, editedSignals?)   ← durable wait state
  Activity: AssembleContext(applicationID, signals) → context
  Activity: RunFitScoring(context, signals) → fitAssessment
  Signal:   HumanApproval(approved)                   ← durable wait state
  Activity: GenerateResume(context) → resumeJSON
  Activity: ReviewProvenance(resumeJSON, context) → provenanceReport
  Activity: ReviewOverstatement(resumeJSON, context) → overstatementReport
  Activity: ReviewToneAccuracy(resumeJSON, context) → toneReport
  Signal:   HumanApproval(approved, flaggedItems?)    ← durable wait state
            (UI shows resume + all three reviewer reports together;
             human adjudicates flagged items, not a blind full read)
  Activity: RenderDocx(resumeJSON) → docxBytes
  Activity: StoreResumeVersion(applicationID, resumeJSON, docxBytes)
```

`HumanApproval` is a Temporal Signal (not a timeout-based sleep or polling
loop) — the workflow sits durably at each wait state until the UI sends the
signal, surviving process restarts cleanly.

**Application status pipeline** (natural CRM layer):
```
discovered → reviewed → applied → screen → interview → offer → closed
```
Timestamps on each transition. Outcome data (screen / no screen) feeds
fit-scoring calibration over time.

---

### Phase 3.5 — Career threads (narrative through-lines)
*Can be built alongside or after Phase 3; depends on career data being well-seeded.*

Career threads are named narrative patterns that emerge from contribution data
spanning multiple positions and employers — "Technical Leadership," "Distributed
Systems Architecture," "Platform Reliability" — as first-class schema objects
rather than implicit tag co-occurrences. They serve resume summaries, fit
scoring context, and eventually interview prep.

**Key design decisions:**
- System-proposed, human-confirmed — never user-authored from scratch. The
  cognitive burden of "tell me your through-line stories" is too high for a
  general user. Recognition ("yes, that's me") is the right UX entry point,
  not creation.
- LLM-driven discovery using the existing tag taxonomy as grounding structure.
  No hardcoded enumeration of thread types — that would miss career-specific
  patterns (e.g. shipboard/embedded distributed systems spanning Lockheed →
  Disney) that a generic list would never anticipate.
- Few-shot examples in the discovery prompt for output format calibration, not
  as an exhaustive list of possible thread types.
- Triggered on-demand after seeding, with a "refresh suggestions" button.
  Background continuous job not warranted at this data change frequency.
- Freetext user creation deliberately not supported in v1 — grounded suggestion
  flow prevents users from creating linkages that produce bogus narratives.

**Discovery prompt requirements:**
- Identify co-occurrence patterns across employers, not just within one position
- Weight recency appropriately (early-career abandoned threads shouldn't dominate)
- Produce a proposed name + one-sentence summary per thread readable by a
  non-technical user — "yes, that's me" is the quality signal
- Return structured JSON for the confirm/dismiss UI

**Schema additions:**
```sql
CREATE TABLE career_threads (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id),
    name         TEXT NOT NULL,
    summary      TEXT,           -- human-edited or LLM-proposed; confirmed by user
    is_active    BOOLEAN NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    confirmed_at TIMESTAMPTZ     -- null = still a suggestion; set on user confirmation
);

CREATE TABLE career_thread_contributions (
    thread_id       UUID NOT NULL REFERENCES career_threads(id),
    contribution_id UUID NOT NULL REFERENCES contributions(id),
    display_order   INT,         -- intentional narrative sequencing
    notes           TEXT,        -- why this contribution belongs in this thread
    PRIMARY KEY (thread_id, contribution_id)
);
```

**Integration points:**
- Resume summary generation: confirmed threads with human-edited summaries give
  Stage 2 a richer, pre-validated source than inferring patterns from raw
  contributions at generation time
- Fit scoring: thread names and summaries feed the preference/domain match
  dimension of the go/no-go gate
- Interview prep (Phase 4): "tell me about a time you showed X" maps directly
  to a thread, not an individual contribution

---

### Phase 4 — Richer features (post-validation)
*Gate: at least one non-me user finds the system genuinely useful over time.*

- **Cover letter generation:** same two-stage pattern, different Stage 2
  template, `cover_letter_versions` table; nearly free given existing pipeline
- **Interview prep:** likely questions mapped to selected contributions for a
  specific role, from JD signals + `resume_version` contribution set
- **Outcome tracking:** screen/offer/reject tied to which contributions were
  selected and fit score — training signal for go/no-go calibration; this is
  also what makes the "personal career knowledge base" framing sticky
- **Prompt evaluation framework:** compare generation quality across prompt
  versions; fixed JD/expected-output test pairs minimum
- **Blob storage interface:** for rendered documents once volume warrants it
- **Multi-user / hosted product:** the `user_id` schema scaffolding is already
  in place; this is a deployment and auth decision, not a schema redesign

---

## Known Skill Gaps (Job Search Context)

Tracked for honest fit assessment; not to be invented into resumes:

| Gap | Status |
|---|---|
| Angular | Real but thin exposure, not zero signal — still a hard pass as a co-equal frontend requirement |
| React | Thin/old (Dignitas co-development); actively closing — the Role Model frontend is built and shipping (auth shell, application generation flow, Vitest coverage) |
| Python at expert/production level | Real but thin: AEMWAS, The Budgeteer, plus the `docx-renderer` FastAPI service |
| OIDC/SAML/OAuth2 design ownership | JWT validation present; auth design not owned |
| Go at production API depth | Substantially closed — the Role Model backend is built: chi router, ~60 endpoints, pgx/sqlc, JWT auth, LLM pipeline, integration tests |
| MongoDB | No signal. The confirmed NoSQL experience is **DynamoDB, not MongoDB** — Edward Jones via Daugherty, Global Tables and cross-region caching, integration context. Cassandra at Manifold is the other NoSQL data point. Do not let "NoSQL" on a JD read as MongoDB. |
| TypeScript/Node | ~4 weeks at Pelotech; surfaces only for roles explicitly requiring it |
| Flutter | Appears in Disney prose; excluded — support context only, not a skill claim |
| Agent frameworks (LangGraph) | No hands-on experience yet; onboarding agent (Phase 1) is the planned closure path — LangGraph specifically, decision final |

---

## Hard-Pass Filters (Job Search Pipeline)

Established across ~35+ JDs processed:
- TypeScript/Node.js as the primary required language
- Expert-level Python as the primary required language
- Ruby/Rails as the primary required language
- C# / .NET as the primary stack
- Angular as a co-equal frontend requirement
- Production LLM/AI-feature experience as a hard requirement — **no longer a
  blanket exclude; see "AI requirements — three categories" below**
- Defense-coded / clearance-required work (cultural fatigue). **Commercial
  aerospace is not excluded** — the filter is the clearance-and-culture shape,
  not the industry
- Big Four consulting roles
- Full-stack roles where frontend depth is co-equal to backend
- Crypto/blockchain product companies
- Onsite outside Orlando, FL

Exception pattern: "use AI tools to build faster" framing at a product company
does not trigger the LLM hard-pass — Disney pilot and Role Model itself satisfy
this framing without requiring production AI feature ownership.

**AI requirements — three categories.** The single "production LLM experience"
line collapsed three different situations that need different handling:

1. **FDE-shaped roles** — deploying and operating agents against a customer's
   real business problem. A conscious exception, evaluated on its own track
   rather than filtered. Bridge story: Manifold/RBC embedded customer work plus
   Lockheed direct customer interaction.
2. **Agent orchestration / runtime / framework platform roles** — where the
   deliverable *is* the agent-building infrastructure and the customers are
   other engineers. Still excluded, but on **values** grounds
   (product-over-platform), not skill grounds. Closing the skill gap does not
   reopen this category.
3. **"AI-production-required" as a qualifications-line technicality** on an
   otherwise-normal role — softened from a blanket Stage 1 auto-fail to Stage 2
   case-by-case judgment. Gated on the MCP server and eval harness actually
   shipping; until then there is nothing to point at.

Learning agent frameworks (LangGraph/CrewAI), multi-agent coordination patterns,
and building the Temporal integration are skill-acquisition moves supporting
category 1. Note that the exclude in category 2 is scoped to **who consumes the
output**, not to any particular technology — the same LangGraph skill that
serves category 1 does not make category 2 acceptable.

**This list has diverged from the seeded `preferences` rows** and the two are
not currently reconcilable by hand:
- The seed stores `domain / defense / aerospace / hard_exclude`, which is the
  broader industry framing this list has now narrowed
- The seed stores `domain / fintech / blockchain` as a **positive** at weight 5
  ("Manifold experience; not a primary target but not a negative either"),
  which directly contradicts the crypto/blockchain hard-pass above
- Ruby/Rails, C#/.NET, and the Orlando onsite constraint have no preference
  rows at all, so nothing in `fitgate` can act on them
- The three-category AI breakdown above is **not representable in the current
  `preferences` schema at all.** A conscious exception evaluated on its own
  track, an exclude held on values rather than skill grounds, and a
  Stage-1-to-Stage-2 downgrade gated on unshipped work are three different
  shapes; `fitgate` knows only flat `hard_exclude` at a weight. This is a
  scoring-model gap, not a missing row.

Reconciling the seeded rows is a data change and stays deferred. The
representability gap is a `fitgate` change and belongs in Phase 2 planning —
neither is silently edited here.

---

## Open Questions

1. **Notification channel** for the Phase 3 notification consumer — email,
   webhook to push service, or something else? Decide before building consumer.
2. **Stage 0 review UI shape** — single-page side-by-side bulk approve, or
   step-by-step per contribution? Recommendation: single-page with bulk approve
   and per-row override.
3. ~~**Fit scoring rubric**~~ — RESOLVED. Shipped in `internal/fitgate/scorer.go`:
   deterministic technical + preference scoring in Go with 1–10 preference
   weights, `fit_reports` persistence (migrations 007/009/010), and the LLM
   confined to writing narrative from the computed scores. Successor question:
   calibration against real outcome data, which needs `applications` rows
   carrying screen/no-screen results that do not exist yet.
4. **Prompt-steering accumulation table** — schema TBD; defer until feedback UI
   is actively being built but track as a known gap.
5. **Lever adapter** — verify `api.lever.co/v0/postings/{company}?mode=json`
   against a real target slug before writing the adapter.
6. ~~**Skills + preferences schema design session**~~ — RESOLVED, both tables
   built and queryable (migrations 005/008/009). Successor question: how do
   `skills.proficiency` / `years_experience` / `skill_provenance` actually get
   populated? Nothing writes to them today, which is why generation still can't
   tell a prototype from production depth.
7. ~~**`contribution_drafts` writethrough flow**~~ — RESOLVED as recommended.
   `stage0.ApproveDraft` holds rows in `contribution_drafts` until an explicit
   `POST /api/v1/import/drafts/{draftID}/approve`, which verifies parent-position
   ownership and writes through to `contributions` in a transaction. Only the
   review *UI* remains unbuilt — the flow behind it is settled (see question 2).
8. **MCP process boundary** — separate process alongside the chi API (own pgx
   pool, and a decision about importing vs. duplicating the sqlc code) or a
   second protocol surface in the same binary (cleaner sharing, different
   deployment shape)? Decide during implementation, not now. See the MCP Server
   section.
9. **MCP tools vs. Temporal workflows** — if Phase 3 lands and the pipeline is
   orchestrated by Temporal, do MCP tools trigger workflows or stay below that
   layer? Blocked on Temporal actually being built. See the MCP Server section.

