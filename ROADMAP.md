# Role Model — Architecture Checkpoint & Roadmap

**Last updated:** June 2026  
**Repo:** github.com/shurikai/role-model  
**Data repo:** private, accessed via `SEED_DIR` env var from `make seed`  
**Companion docs:** `docs/discovery-design.md`

---

## System Identity

Role Model is a self-hostable, AI-powered career management platform. Its core
thesis: a person's career history, structured as a queryable knowledge base of
contribution "atoms," is the raw material for all downstream job search
activities — resume generation, fit assessment, cover letters, interview prep.

The LLM layer synthesizes and personalizes. SQL retrieves and filters. These
roles are not interchangeable, and the architecture enforces that separation.

This is bespoke RAG: deterministic SQL retrieval driven by Stage 1 LLM signal
extraction. LangChain-style orchestration, the Claude Agent SDK, and general
agentic loop patterns are all explicitly out of scope — the pipeline is
sequential, human-in-the-loop, and deterministic by design.

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
| Frontend (planned) | React + TypeScript + Vite |

---

## Implementation Checkpoint — What Is Built and Verified

### Infrastructure
- Docker Compose managing PostgreSQL 16 on host port 5433
- golang-migrate: 4 migrations applied
- Makefile with `db-up`, `db-down`, `migrate`, `seed`, `reset` targets
- `make seed` reads `SEED_DIR` and applies SQL files in order
- `internal/httputil` package (resolved the handlers↔middleware circular import)

### Schema — 18 tables
`users`, `employers`, `positions`, `contributions`, `tags`, `contribution_tags`,
`position_tags`, `education`, `education_tags`, `credentials`, `projects`,
`project_tags`, `applications`, `jd_signals` (JSONB), `resume_versions`,
`contribution_feedback`, `generation_params` (JSONB), `structured_output` (JSONB)

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

### Seed Data — 12 files, complete canonical career history
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
```

**Pending seed tasks (tracked in memory):**
1. Verify DynamoDB tag and contribution text in `003_daugherty.sql` are accurate
   for the Edward Jones engagement (DynamoDB Global Tables, cross-region caching,
   integration-context framing — same level as Cassandra at Manifold)
2. Verify and add Groovy tag to active Manifold contribution once canonical
   story is confirmed with "built and deployed in one week" detail

### Authentication
- bcrypt password hashing
- 24-hour JWT tokens (single token per user)
- `RequireAuth` middleware; all protected routes carry real authenticated identity
- `stubUserID` fully replaced with context-based identity

### API Endpoints — Built and Verified
```
GET  /health

POST /api/v1/auth/register
POST /api/v1/auth/login

GET    /api/v1/employers
POST   /api/v1/employers
GET    /api/v1/employers/:id
PUT    /api/v1/employers/:id
DELETE /api/v1/employers/:id          (guarded)

GET    /api/v1/employers/:id/positions
POST   /api/v1/employers/:id/positions
GET    /api/v1/positions/:id
PUT    /api/v1/positions/:id
DELETE /api/v1/positions/:id          (guarded)

GET    /api/v1/positions/:id/contributions
POST   /api/v1/positions/:id/contributions
GET    /api/v1/contributions/:id
PUT    /api/v1/contributions/:id
DELETE /api/v1/contributions/:id      (transactional join-table cleanup)

POST   /api/v1/applications
GET    /api/v1/applications
GET    /api/v1/applications/:id
PUT    /api/v1/applications/:id
DELETE /api/v1/applications/:id

POST   /api/v1/applications/:id/extract    — Stage 1: JD → jd_signals JSONB
POST   /api/v1/applications/:id/generate   — Stage 2: signals + context → resume_version
GET    /api/v1/applications/:id/versions
GET    /api/v1/resume-versions/:id
```

### Two-Stage LLM Pipeline — Built and Verified
**Stage 1 — JD Signal Extraction**
- Input: raw JD text
- Anthropic API call against `jd_extraction.v1.tmpl`
- Output: `jd_signals` JSONB stored on `applications` row
- Verified against real job descriptions

**Stage 2 — Resume Generation**
- `AssembleContext`: composes employer/position/contribution/project/education/
  credential data via sqlc queries into the structure the prompt expects
- Renders `resume_generation.v1.tmpl`
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

## Schema Gaps — Identified, Not Yet Built

These are confirmed design decisions parked for a dedicated session:

### Skills table (proposed)
Skills are currently emergent from contribution tags — there is no standalone
record expressing "Java, 25 years, expert" as a fact independent of the
contributions that prove it. Provenance exists; the fact itself does not.

Proposed additions:
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

### Preferences table (proposed)
Preferences currently live only in conversation memory and in the pattern of
job-search decisions made across sessions. They are not queryable by the
pipeline. This is a real gap for fit scoring.

Proposed additions:
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

Known preference profile (from memory, needs to be seeded once table exists):

| Type | Sentiment | Label |
|---|---|---|
| domain | positive | distributed systems |
| domain | positive | IoT / telemetry / real-time data |
| domain | positive | consumer-facing product |
| work_type | positive | product over platform/internal tooling |
| work_type | positive | small team, high ownership |
| work_type | positive | greenfield over pure maintenance |
| culture | positive | remote-first |
| culture | positive | low-ego, async |
| anti_pattern | hard_exclude | Big Four consulting |
| anti_pattern | hard_exclude | defense / aerospace |
| anti_pattern | hard_exclude | pure frontend |
| anti_pattern | hard_exclude | TypeScript/Node as primary language |
| anti_pattern | hard_exclude | expert Python as primary requirement |
| anti_pattern | hard_exclude | production LLM/AI as hard requirement |
| anti_pattern | hard_exclude | Angular as co-equal frontend requirement |
| anti_pattern | negative | full-stack where frontend is co-equal |
| anti_pattern | negative | platform/internal tooling over product |
| culture | negative | military-coded culture |

---

## Roadmap

### Phase 1 — Usable by humans other than me
*Gate: two non-technical test users can enter career data and get a resume
without touching the terminal.*

**Backend remaining:**
- Projects, education, credentials write-CRUD (read endpoints exist; write not yet built)
- Pending seed tasks (MySQL tag, Groovy verification)
- Skills + preferences schema and seed (design session needed first)

**Stage 0 — LLM-assisted data entry pipeline** (new; designed this session)  
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

**Frontend:**
- React + TypeScript + Vite (decided)
- AppShell + SidebarNav first build targets
- QuickLaunchPanel (JD paste/submit) + RecentApplicationsTable on dashboard
- Data-entry UI must be designed with Stage 0 review flow in mind from the start
  — do not design plain forms and retrofit LLM-assist later

**Auth + deploy:**
- Google OAuth (replace bcrypt/JWT stub)
- Fly.io deployment for two external test users

---

### Phase 2 — Complete end-to-end pipeline
*Gate: JD input → .docx download without leaving the UI.*

1. Stage 1 + Stage 2 fully wired as API endpoints with UI surfaces
2. Human-in-the-loop signal review gate (UI for reviewing jd_signals before
   generation runs — this is a product requirement, not just a nice-to-have)
3. **Go/no-go fit gate** — deterministic scoring pass before Stage 2 runs:
   - Score dimensions: tech stack coverage (tag matching), level match,
     domain match, gap list
   - LLM writes prose narrative from the deterministic scores; it interprets,
     it does not score
   - Output: scored breakdown + recommendation (strong / marginal / hard pass)
   - Human reviews fit assessment and explicitly proceeds to generation
   - Schema: `fit_assessments` table linked to `applications`
   - Implementation note: deterministic v1 first; calibrate against outcome
     data once enough `applications` rows exist with real outcomes recorded
4. **Feedback loop:**
   - Two levels: whole-resume and per-contribution
   - `feedback_type` enum: `correction` | `selection` | `phrasing`
     - `correction`: wrong at source; propagate upstream to canonical contribution
     - `selection`: right contribution, wrong role; feeds selection heuristics
     - `phrasing`: correct content, wrong wording; feeds prompt-steering table
   - Prompt-steering accumulation: separate table injected into future generation
     calls (schema TBD, defer until feedback UI is in scope)
5. Renderer (docx output):
   - Python-based, `resume.v1.json` → `.docx`
   - In Phase 2: runs as a synchronous step or simple worker
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

**`cmd/discovery` worker** (see `docs/discovery-design.md` for full design):
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
| Angular | Zero signal — hard pass on roles requiring it |
| React | Thin/old (Dignitas co-development); Role Model frontend will close this |
| Python at expert/production level | Real but thin: AEMWAS, The Budgeteer |
| OIDC/SAML/OAuth2 design ownership | JWT validation present; auth design not owned |
| Go at production API depth | Role Model closes this gap when complete |
| MongoDB/NoSQL | DynamoDB confirmed at Edward Jones (Global Tables, integration context) |
| TypeScript/Node | ~4 weeks at Pelotech; surfaces only for roles explicitly requiring it |
| Flutter | Appears in Disney prose; excluded — support context only, not a skill claim |

---

## Hard-Pass Filters (Job Search Pipeline)

Established across ~35+ JDs processed:
- TypeScript/Node.js as the primary required language
- Expert-level Python as the primary required language
- Angular as a co-equal frontend requirement
- Production LLM/AI-feature experience as a hard requirement
- Defense / aerospace domain (cultural fatigue)
- Big Four consulting roles
- Full-stack roles where frontend depth is co-equal to backend

Exception pattern: "use AI tools to build faster" framing at a product company
does not trigger the LLM hard-pass — Disney pilot and Role Model itself satisfy
this framing without requiring production AI feature ownership.

---

## Open Questions

1. **Notification channel** for the Phase 3 notification consumer — email,
   webhook to push service, or something else? Decide before building consumer.
2. **Stage 0 review UI shape** — single-page side-by-side bulk approve, or
   step-by-step per contribution? Recommendation: single-page with bulk approve
   and per-row override.
3. **Fit scoring rubric** — define explicit dimension weights and score ranges
   before building the go/no-go gate; first version must be deterministic and
   auditable.
4. **Prompt-steering accumulation table** — schema TBD; defer until feedback UI
   is actively being built but track as a known gap.
5. **Lever adapter** — verify `api.lever.co/v0/postings/{company}?mode=json`
   against a real target slug before writing the adapter.
6. **Skills + preferences schema design session** — needed before Phase 2 fit
   scoring work begins; both tables must be queryable by the pipeline.
7. **`contribution_drafts` writethrough flow** — does the Stage 0 review UI
   write directly to `contributions` or hold in `contribution_drafts` until a
   final confirm step? Recommendation: hold in drafts with explicit confirm;
   keeps canonical data clean.

