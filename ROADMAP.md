# Role Model — Architecture Checkpoint & Roadmap

**Last updated:** July 2026
**Repo:** github.com/shurikai/role-model
**Data repo:** private, accessed via `RESUMATRON_DATA_DIR` from `make seed`
**Companion docs:** `CLAUDE.md`, `docs/discovery-design.md`

> Living document. This is the architectural source of truth for session
> continuity and the pick-up point for design discussion. Update the Current
> State section as work lands.

---

## System Identity

Role Model is a self-hostable, AI-powered career management platform. Its core
thesis: a person's career history, structured as a queryable knowledge base of
contribution "atoms," is the raw material for all downstream job-search
activities — resume generation, fit assessment, cover letters, interview prep.

The LLM layer synthesizes and personalizes. SQL retrieves and filters. These
roles are not interchangeable, and the architecture enforces the separation.
This is bespoke RAG: deterministic SQL retrieval driven by Stage 1 LLM signal
extraction, chosen over vector search for auditability in the domain of
professional self-representation.

**Positioning:** a personal career knowledge base, not a resume generator. The
distinction matters for retention (the system compounds in value over time) and
for any future monetization framing (the structured career data is the moat,
not the prompts).

---

## Current State (July 2026)

### Data & schema
- PostgreSQL schema, migrations `001`–`005`:
  - `001` initial (employers, positions, contributions, applications,
    resume_versions, tags, tag_categories, projects, education, credentials,
    and all join tables)
  - `002` position location
  - `003` user identity fields
  - `004` password_hash (nullable, OAuth-ready)
  - skills + preferences schema (skills referencing canonical `tags.tag_id`;
    `v_skill_provenance` as a view; preferences with positive/negative/hard_exclude
    sentiment and nullable `context_type`; all enum-likes as TEXT + CHECK)
- Seed data via private repo, stable UUIDs, upsert semantics, files through
  `015_mysql.sql` (career history, tags, projects, education, skills,
  preferences, MySQL tag linked to WARSIM/L0005)

### Pipeline
- **Stage 1 — JD signal extraction:** working. Extracts priority skills,
  seniority, domain vocabulary; stored as JSONB on the application.
- **Stage 2 — resume generation:** working end to end. AssembleContext gathers
  full career context (experience, projects, education, credentials) skipping
  positions with no active contributions; renders the generation prompt; calls
  the Anthropic API; injects generator-owned provenance (real model name,
  prompt version, ids, timestamps); validates against the formal
  `resume.v1.json` JSON Schema; stores an immutable versioned `resume_version`.
  Full contribution/position/project ID traceability through generated output.
- **Stage 0 — LLM-assisted import:** built (`import_batches`,
  `contribution_drafts`; Stage 0a structural extraction, Stage 0b per-bullet
  enrichment with a hard constraint against inventing numbers; human-review API:
  POST /import, GET /import/{batchID}, GET /import/{batchID}/drafts,
  GET/PUT /import/drafts/{id}, approve/reject). Write-through to canonical
  contributions only on explicit human approval.
- **Fit gate & scoring:** built (`fit_reports`; deterministic anti-pattern hard
  gate first; technical fit and preference fit scored in parallel for anything
  that passes; LLM produces narrative, not the score; two-score output).
  `fit_reports.application_id` nullable to allow pre-application fit checks;
  reports immutable (re-runs create new rows). Handlers under
  `/applications/{applicationID}/fit`.

### Platform
- JWT auth: signup/login, bcrypt, 24h single tokens, `RequireAuth` middleware,
  standalone `IssueToken`/`ParseToken` (signing-method enforced) — designed so
  OAuth slots in later via a future `user_identities` table. `stubUserID` fully
  removed; identity flows from token through request context.
- Read endpoints: employers, positions, contributions; application CRUD;
  resume version reads.
- **Write CRUD (complete):** employers, positions, contributions, education,
  credentials — create/update/delete, full-replace updates, guarded deletes
  (409 on dependents for employer/position; transactional join cleanup for
  contribution), parent-ownership checks on nested creates, all scoped to the
  authenticated user.
- Middleware: panic recovery (JSON error shape) + request logging.
- Shared `internal/httputil` for response helpers and user-context accessors
  (resolved the earlier handlers/middleware import cycle).
- API integration test harness (`httptest` against the real router + test DB,
  behind the `integration` build tag): multi-tenant isolation test, contribution
  transactional-delete test, and CRUD coverage growing per resource.

### Conventions
- CLAUDE.md as session source of truth; GitHub Issues with label taxonomy;
  destination-oriented Claude Code session prompts, numbered and tracked.
- Client-generated UUIDs; TEXT+CHECK over native enums; nullable JSONB via
  column-level sqlc overrides (`*json.RawMessage`) to serialize as JSON not base64.

---

## Remaining Write CRUD (in flight)
- **Tags + categories + assignment** — tag/category create; join-table
  assignment endpoints (conflict-tolerant), usage-guarded deletes. (New route
  shape: `POST /contributions/{id}/tags`, etc.)
- **Projects** — full CRUD plus both join types (contributions and tags),
  reusing the transaction pattern from contributions and the assignment pattern
  from tags. The convergence slice.

---

## Phase 1 — Usable by non-technical test users
**Goal:** two or three non-technical people (incl. user #2, pre-seeded; user #3,
self-serve from zero) can use the system to evaluate whether it's useful.

- [x] Auth / account creation
- [x] Write CRUD + data-entry endpoints (employers/positions/contributions/
      education/credentials done; tags + projects remaining)
- [x] Stage 0 LLM-assisted data entry (lowers the barrier for non-technical
      users entering rich career data — protects downstream generation quality)
- [ ] Frontend: auth screens → data-entry UI → dashboard (React + TS + Vite,
      dev-tool aesthetic, sidebar shell)
- [ ] Renderer: intermediate resume JSON → human-readable PDF/DOCX (the output
      a test user actually judges). Interface already defined
      (`Render(json) → bytes`); likely HTML→PDF first pass, python-docx for
      editable output later.
- [ ] Hosted deployment (Fly.io) so external users can reach it

---

## Phase 2 — Complete end-to-end pipeline
- [x] Fit gate / go-no-go with technical + preference scoring
- [ ] Temporal integration for the sequential, human-in-the-loop generation
      pipeline with durable wait states (Signals for human approval gates)
- [ ] Feedback loop implementation (schema exists: correction / selection /
      phrasing types). `prior_feedback` currently passed empty into generation;
      wire accept/reject/edit signal back into future generations.
- [ ] Stage 0b tag suggestion (extend enrichment to suggest tag_ids from
      canonical vocabulary; approve endpoint accepts optional tag_ids)

---

## Phase 3 — Discovery & distributed systems
- [ ] Discovery worker (`cmd/discovery`): ATS scrapers producing
      `job_posting.discovered` events. Adapter interface per ATS platform;
      `companies.yaml` config; Kafka fan-out to extraction, notification, and
      future filtering consumers. Confirmed public feeds: Greenhouse
      (boards-api.greenhouse.io), Ashby (api.ashbyhq.com/posting-api/...),
      Lever (pattern identified, unverified). Design in `docs/discovery-design.md`.
- [ ] Kafka event taxonomy; event types defined once in shared go.mod.

---

## Phase 4 — Post-validation features
- [ ] CC pipeline orchestration: feed a batch of JDs, get ranked
      apply/pass recommendations with reasoning, human selection, then generated
      resumes for chosen roles. Drives the existing REST API via bash (no MCP
      required); depends on the fit-assessment stage (done) and renderer being
      complete. Turns the manual batch-JD triage into automated-with-human-gates.
- [ ] Cover letters, LinkedIn content, interview prep — each stress-tested
      against "does this make sense if resume generation didn't exist," to keep
      the core product honest.
- [ ] Monetization: tier structure (active job seeker vs. continuous career-record
      maintainer), subscription economics, the episodic-churn problem. Kept as a
      background prioritization lens, not actively resolved.

---

## Standing Design Principles
- LLM interprets and synthesizes; it never stores, retrieves, or scores.
- The more dangerous LLM failure is confident plausible overstatement, not
  outright fabrication — guard against drift ("contributed to" → "led").
- Never invent absent fields (location, metrics, dates) — omit and flag.
- Timeline gaps are worse than weak entries; include full history.
- Inactive contributions (e.g. Pelotech) stay invisible to generation by design.
- Kafka and Temporal each earn their place against a real use case, not resume
  decoration.
- Keep the LLM layer behind a clean interface; no self-hosting/fine-tuning
  planning needed until API cost at scale demands it.

---

## Deferred / Explicitly Parked
- `generation_instruction` concept (documented to prevent re-litigation)
- Refresh-token flow (single token is sufficient at current scale)
- Dedicated test database (integration tests currently share the dev DB and are
  starting to collide — promote to a real need soon)
- `RouterDeps` → `Deps` rename (cosmetic)
- Richer location modeling (single TEXT field sufficient for now)
