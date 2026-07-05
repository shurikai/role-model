# Role Model — Architecture Checkpoint & Roadmap

**Last updated:** July 2026
**Repo:** github.com/shurikai/role-model
**Data repo:** private, accessed via `RESUMATRON_DATA_DIR` from `make seed`
**Companion docs:** `CLAUDE.md`, `docs/discovery-design.md`

> Living document. Architectural source of truth for session continuity and the
> pick-up point for design discussion. Update the Current State section as work lands.

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
- PostgreSQL schema with migrations for: employers, positions, contributions,
  applications, resume_versions, tags, tag_categories, projects, education,
  credentials, and all join tables; position location; user identity fields;
  password_hash (nullable, OAuth-ready); skills + preferences.
- Skills reference canonical `tags.tag_id`; `v_skill_provenance` is a view.
  Preferences use positive/negative/hard_exclude sentiment + nullable
  `context_type`. All enum-likes are TEXT + CHECK, not native enums.
- Seed data via private repo, stable UUIDs, upsert semantics, files through
  `015_mysql.sql`.
- **Known data gap:** `project_contributions` join is empty — no project has
  linked contributions yet (see backlog). Projects are correctly skipped in
  generation until seeded.

### Pipeline
- **Stage 0 — LLM-assisted import:** built. `import_batches`,
  `contribution_drafts`; Stage 0a structural extraction, Stage 0b per-bullet
  enrichment (hard constraint against inventing numbers); human-review API
  (POST /import, GET /import/{batchID}, .../drafts, GET/PUT /import/drafts/{id},
  approve/reject). Write-through to canonical contributions only on approval.
- **Stage 1 — JD signal extraction:** working. Stored as JSONB on the application.
  (Struct expanded during fit-gate work for required/preferred skill distinction.)
- **Stage 2 — resume generation:** working end to end. AssembleContext gathers
  full career context; skips positions with no active contributions AND projects
  with no contributions. Generator-owned provenance (real model name, prompt
  version, ids, timestamps). Validates against formal `resume.v1.json` JSON
  Schema. Immutable versioned `resume_version`. Full contribution/position/
  project ID traceability through output (every bullet cites its source ids).
- **Fit gate & scoring:** built. `fit_reports`; deterministic anti-pattern hard
  gate first; technical + preference fit scored in parallel for anything that
  passes; LLM writes narrative, not the score. Two-score output.
  `fit_reports.application_id` nullable (pre-application checks); reports
  immutable. Handlers under `/applications/{applicationID}/fit`.

### Platform
- JWT auth: signup/login, bcrypt, 24h single tokens, `RequireAuth` middleware,
  standalone `IssueToken`/`ParseToken` (signing-method enforced). OAuth-ready via
  future `user_identities` table + nullable password_hash. `stubUserID` removed;
  identity flows from token through request context.
- **Read endpoints:** employers, positions, contributions; application CRUD;
  resume version reads.
- **Write CRUD (complete for 5 of 7 resources):** employers, positions,
  contributions, education, credentials — create/update/delete. Full-replace
  updates. Guarded deletes (409 on dependents for employer/position;
  transactional join cleanup for contribution — cleans contribution_tags,
  project_contributions, AND contribution_feedback). Rows-affected checks so
  not-owned deletes return 404, not false 204. Parent-ownership checks on nested
  creates. All scoped to authenticated user.
- Middleware: panic recovery (JSON error shape) + request logging.
- Shared `internal/httputil` for response helpers + user-context accessors.
- Contribution delete extracted into a `contribution.Service` (business logic
  out of the handler); other deletes remain inline — two patterns coexist.

### Testing
- API integration harness (`httptest` against real router + test DB, behind the
  `integration` build tag).
- Coverage: multi-tenant isolation (employers + education/credentials),
  contribution transactional-delete, education/credentials CRUD, AssembleContext.
- Test helper `createAndGetID` reads response body once and prints body on
  failure (avoids the earlier double-close bug).
- **Known hazard:** `testServer` builds its own `RouterDeps` literal, hand-synced
  with `main.go`. It already drifted once (omitted `ContribSvc` -> nil-service
  panic). `Stage0Svc`/`FitSvc` are similarly at risk. See backlog: unify Deps
  assembly or add nil-checks in NewRouter.

### Conventions
- CLAUDE.md as session source of truth; GitHub Issues with label taxonomy;
  destination-oriented Claude Code session prompts, numbered and tracked.
- Client-generated UUIDs; TEXT+CHECK over native enums; nullable JSONB via
  column-level sqlc overrides (`*json.RawMessage`).
- Delete-transaction rule: clean up EVERY table with a FK referencing the target.
  Enumerate with the information_schema FK query before writing the transaction.

---

## Remaining Write CRUD (in flight)
- **Tags + categories + assignment** — tag/category create; join-table
  assignment endpoints (conflict-tolerant via ON CONFLICT DO NOTHING),
  usage-guarded deletes. New route shape: `POST /contributions/{id}/tags`, etc.
  Double-ownership check on assignment (both the parent and the tag must belong
  to the user).
- **Projects** — full CRUD plus both join types (contributions and tags),
  reusing the transaction pattern from contributions and the assignment pattern
  from tags. The convergence slice.

---

## Phase 1 — Usable by non-technical test users
**Goal:** two or three non-technical people (user #2 pre-seeded; user #3 self-serve)
can use the system to evaluate whether it's useful.

- [x] Auth / account creation
- [x] Write CRUD for employers/positions/contributions/education/credentials
- [ ] Write CRUD for tags + projects (in flight — last two resources)
- [x] Stage 0 LLM-assisted data entry (lowers barrier for non-technical users)
- [ ] Frontend: auth screens -> data-entry UI -> dashboard (React + TS + Vite,
      dev-tool aesthetic, sidebar shell)
- [ ] Renderer: resume JSON -> human-readable PDF/DOCX. Interface defined
      (`Render(json) -> bytes`); likely HTML->PDF first, python-docx for editable later.
- [ ] Hosted deployment (Fly.io)

---

## Phase 2 — Complete end-to-end pipeline
- [x] Fit gate / go-no-go with technical + preference scoring
- [ ] Temporal integration for the sequential, human-in-the-loop generation
      pipeline with durable wait states (Signals for approval gates)
- [ ] Feedback loop implementation (schema exists: correction/selection/phrasing).
      `prior_feedback` currently passed empty into generation; wire accept/reject/
      edit signal back into future generations.
- [ ] Stage 0b tag suggestion (suggest tag_ids from canonical vocabulary; approve
      endpoint accepts optional tag_ids)

---

## Phase 3 — Discovery & distributed systems
- [ ] Discovery worker (`cmd/discovery`): ATS scrapers producing
      `job_posting.discovered` events. Adapter interface per ATS platform;
      `companies.yaml` config; Kafka fan-out to extraction, notification, and
      future filtering consumers. Public feeds confirmed: Greenhouse
      (boards-api.greenhouse.io), Ashby (api.ashbyhq.com/posting-api/...),
      Lever (pattern identified, unverified). Design in `docs/discovery-design.md`.
- [ ] Kafka event taxonomy; event types defined once in shared go.mod.

---

## Phase 4 — Post-validation features
- [ ] CC pipeline orchestration: feed a batch of JDs, get ranked apply/pass
      recommendations with reasoning, human selection, then generated resumes.
      Drives the existing REST API via bash (no MCP required); depends on the
      fit-assessment stage (done) and the renderer. Turns manual batch-JD triage
      into automated-with-human-gates.
- [ ] Cover letters, LinkedIn content, interview prep — each stress-tested against
      "does this make sense if resume generation didn't exist," to keep the core
      product honest.
- [ ] Monetization: tier structure (active job seeker vs. continuous career-record
      maintainer), subscription economics, the episodic-churn problem. Background
      prioritization lens, not actively resolved.

---

## Backlog (concrete, near-term)
- **Seed project_contributions.** The join is empty; projects produce no citable
  bullets and are skipped in generation. Write contribution atoms for Role Model,
  dedupe, and other projects worth showing; seed the links. Until then, projects
  don't appear on generated resumes. Medium priority — strong portfolio signals
  for target roles, worth doing before serious use.
- **Unify Deps assembly / add nil-checks.** `testServer` and `main.go` build
  `RouterDeps` separately; drift caused a nil-service panic. Extract to a shared
  constructor, or have `NewRouter` fail loudly on nil deps at construction.
- **Dedicated test database.** Integration tests share the dev DB and are starting
  to collide (stray manual rows have broken assembly assertions). Promote to a
  real need before the test count grows further.
- **Converge delete patterns.** Contribution delete uses a service layer; others
  are inline. Decide on one pattern.

---

## Standing Design Principles
- LLM interprets and synthesizes; it never stores, retrieves, or scores.
- The more dangerous LLM failure is confident plausible overstatement, not
  outright fabrication — guard against drift ("contributed to" -> "led").
- Never invent absent fields (location, metrics, dates) — omit and flag.
- Every resume bullet traces to a source contribution (schema enforces
  contribution_ids minItems:1). Positions/projects with no citable contributions
  are skipped, not shown hollow.
- Timeline gaps are worse than weak entries; include full employment history.
- Inactive contributions (e.g. Pelotech) stay invisible to generation by design.
- Kafka and Temporal each earn their place against a real use case.
- Keep the LLM layer behind a clean interface; no self-hosting/fine-tuning
  planning needed until API cost at scale demands it.

---

## Deferred / Explicitly Parked
- `generation_instruction` concept (documented to prevent re-litigation)
- Refresh-token flow (single token sufficient at current scale)
- `RouterDeps` -> `Deps` rename (cosmetic)
- Richer location modeling (single TEXT field sufficient for now)
