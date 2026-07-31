# Role Model

**Repository:** https://github.com/shurikai/role-model

A self-hostable, single-user service for AI-powered resume generation.
Stores detailed career history as structured data and generates tailored,
versioned resumes per job application using an LLM.

Designed with a clear path to multi-tenant deployment without requiring
a schema redesign.

## Status
Active development. Backend: employer/position/contribution CRUD, application
CRUD, tags and tag-categories CRUD, education/credentials/projects, JWT auth
(signup/login/me), JD signal extraction and resume generation pipeline,
Stage 0 import (extract + enrich + approve/reject), fit gate scoring with
skills and preferences tables, CORS. Frontend: auth shell plus the
application generation flow (JD paste → fit → generate → render), built on
Vite + React + TypeScript.

Resume rendering is built: a Python docx-renderer service (FastAPI +
python-docx) owns document output, and internal/renderer is the Go HTTP
client that calls it. Note that internal/renderer was deleted once as dead
code and later reintroduced with an actual consumer — the current package is
the client, not the old renderer implementation.

## Stack
- **Language:** Go
- **Router:** chi
- **Database:** PostgreSQL via pgx/v5 (native interface, not database/sql)
- **Query generation:** sqlc
- **Migrations:** golang-migrate
- **LLM:** Anthropic API via official Go SDK
- **Prompt storage:** internal/generation/prompts directory, embedded via go:embed
- **Prompt templating:** text/template
- **JSON schema validation:** santhosh-tekuri/jsonschema
- **Renderer:** Python 3.14 (FastAPI + python-docx), managed with uv,
  run as a separate process — not part of the Go binary
- **Frontend:** React + TypeScript on Vite, TanStack Query, Vitest

## Architecture

### LLM pipeline
1. **JD signal extraction (Stage 1)** — takes raw job description text, returns
   structured jd_signals JSON (priority skills, seniority, domain vocabulary)
2. **Resume generation (Stage 2)** — split into two calls:
   - **2a body** (`resume_body.v4.tmpl`) — selects and writes bullets and
     skills against jd_signals, under a seniority-informed length budget
   - **2b summary** (`resume_summary.*.tmpl`) — writes the summary scoped to
     the bullets 2a already selected, so it cannot assert unsupported claims

   Facts that both calls would otherwise decide independently (e.g. the header
   title) are threaded through as explicit inputs rather than re-derived. This
   is the established pattern for cross-call consistency — follow it.

Both calls are recorded separately in generation_params for per-call
traceability.

### Fit gate
`internal/fitgate` runs a deterministic scoring pass before generation:
technical coverage plus preference fit, scored in Go. The LLM only writes
prose narrative from those scores — it interprets, it does not score.
Unmet preferences (JD simply doesn't mention something) and genuine conflicts
(JD involves something actively unwanted) are tracked as separate lists and
must stay that way; collapsing them produces false conflict language.

### Intermediate resume JSON
Generation produces a structured JSON document (see /schema/resume.v1.json)
that is the contract between the generation pipeline and the renderer.
The renderer never touches the database. The JSON document is self-contained.

### Renderer
Built. `docx-renderer/` is a small Python service (FastAPI + python-docx)
exposing a single `POST /render` endpoint that takes the intermediate resume
JSON and returns a `.docx`. The Go service reaches it through
`internal/renderer.Client`, surfaced as
`POST /api/v1/resume-versions/{id}/render`.

The renderer never touches the database — the JSON document it receives is
self-contained. Layout is explicit and compact: it does not use Word heading
styles, and it sets `keep_with_next` on section-heading and role-header
paragraph chains for widow/orphan protection. Bullets are deliberately left
free to break across pages.

### Prompt management
Prompts are versioned files in /internal/generation/prompts, embedded into the
binary at compile time via go:embed. Prompt version is recorded in
resume_versions.generation_params so every generated resume can be traced back
to the exact prompt that produced it.

## Project Structure
/cmd/server                      — main entry point
/internal/api/handlers           — HTTP handlers
/internal/auth                   — JWT issuance/validation, bcrypt
/internal/config                 — environment-based config loading
/internal/db                     — sqlc generated code
/internal/fitgate                — deterministic fit scoring + narrative
/internal/generation             — LLM pipeline (signal extraction + resume generation)
/internal/generation/prompts     — LLM prompt template files (embedded at compile time)
/internal/httputil               — shared HTTP helpers (breaks handlers↔middleware cycle)
/internal/renderer               — HTTP client for the docx-renderer service
/internal/stage0                 — LLM-assisted import (extract + enrich + review)
/docx-renderer                   — Python service: resume JSON -> .docx
/frontend                        — React + TypeScript + Vite UI
/database/seed                   — seed SQL scripts
/migrations                      — golang-migrate SQL migration files
/schema                          — JSON schema documents
/tests/fixtures                  — JD, resume JSON, and .docx regression fixtures
/prompts/cc_sessions             — per-session task specs (scaffolding record)
/notes                           — working notes

## Key Files
- /CLAUDE.md                 — project instructions and conventions (this file)
- /schema/resume.v1.json     — intermediate resume JSON schema (source of truth)
- /migrations/               — database migrations (source of truth for schema)
- /internal/generation/prompts/ — LLM prompt templates

## Data Model Decisions
- UUIDs for all primary keys, client-generated
- created_at / updated_at on all tables
- Soft deletes on contributions (is_active) and anywhere historical data has value
- user_id on all tenant-scoped tables (employers, positions, contributions,
  applications, resume_versions, etc.)
- JSONB for flexible blobs: jd_signals, generation_params, structured_output,
  edited_deltas
- Tags are user-defined with user-defined categories, normalized via aliases
- Positions carry both verbatim company title and industry-normalized level/role
  with a level_rationale field
- Contributions are richer than resume bullets: full_description, outcomes, and
  scale_context are separate fields to give the LLM distinct signals to draw from
- Bullet traceability: each generated bullet in the JSON carries contribution_ids
  linking back to source contributions
- Feedback is scoped per resume version, not per contribution globally

## GitHub Issues

This project uses `gh` (GitHub CLI) for issue tracking. The `gh` agent skill is
installed (`gh skill install cli/cli gh --scope user`) and should be used for any
issue or PR interaction rather than constructing raw `gh api` calls from scratch.

### Session workflow

**At session start:**
- If no specific task was given, run `gh issue list --label stage-2` (or the
  relevant label) to find the next queued item rather than assuming there is
  nothing to do.
- When picking up an issue, apply the `in-progress` label:
  `gh issue edit N --add-label in-progress`
- Leave a comment on the issue noting the session date and starting point:
  `gh issue comment N --body "Picking up in session YYYY-MM-DD. Starting from X."`

**During a session:**
- Reference issues in commit messages: `Refs #N` for related work,
  `Closes #N` only when the fix is verified working (tests pass, manually
  confirmed where relevant) — not just written.
- Do not close an issue automatically as part of a larger task unless explicitly
  asked. Surface "this looks like it resolves #N" and let the human confirm.

**At session end:**
- Leave a closing comment summarizing what was done, what was deferred, and
  any blockers: `gh issue comment N --body "..."`
- Remove the `in-progress` label when closing or suspending work:
  `gh issue edit N --remove-label in-progress`
- Close only on explicit human confirmation: `gh issue close N`

### Labels in use
- `stage-2` — resume generation pipeline work
- `renderer` — DOCX/PDF rendering
- `infra` — tooling, migrations, dev environment
- `backlog` — deferred, not forgotten
- `in-progress` — actively being worked in the current or most recent session

Apply an existing label rather than inventing a new one. Ask if a new label
seems genuinely warranted.

### What belongs in Issues vs. here
- Issues are the source of truth for *what's planned and tracked*.
- This file is the source of truth for *how to build it* (stack, conventions,
  the Do Not list below).
- Do not duplicate task lists here that belong in Issues.

## API Design
- REST
- JSON request/response
- Structured error responses, not raw strings
- Environment-based config, no hardcoded values
- JWT-based auth (24h token, no refresh — see internal/auth), single-tenant
  today but every table already carries user_id for a clean path to
  multi-tenant later

## Conventions
- No ORM — use sqlc generated code against pgx native interface
- No database/sql — pgx native only
- No framework beyond chi — stdlib patterns otherwise
- Errors returned as structured JSON: { "error": "message", "code": "slug" }
- All handlers receive a context, all DB calls respect context cancellation
- Config via environment variables, loaded at startup into a typed Config struct

## Do Not
- Use an ORM
- Use database/sql directly
- Use gin, echo, or any heavy framework
- Hardcode any user identity, file paths, or config values
- Add dependencies without a clear justification
- Store rendered document files in the database (blob storage interface goes here)
- Put business logic in HTTP handlers
- Invent prompt improvements — prompts live in /internal/generation/prompts and
  are versioned explicitly (/prompts holds per-session task specs, not prompts)
- Open new issues unprompted during a session focused on something else. If you
  notice unrelated work that should be tracked, mention it and let the human decide.
- Use the `claude-code-action` GitHub App or any webhook-triggered automation.
  All `gh` usage is interactive, inside a human-initiated session only.

## Open Questions
- Blob storage interface for rendered documents (local disk now, S3 later).
  Rendered .docx bytes are currently returned to the caller, not persisted.
- Evaluation strategy for prompt quality across versions (deferred)
- Skill depth signal. The schema supports it — `skills.proficiency` and
  `years_experience` exist, and `v_skill_provenance` derives skill→contribution
  links from `contribution_tags` automatically — but migration 008 backfilled
  every skill at a uniform `proficient` / `NULL`, so the data carries no
  differentiation. A one-off prototype and a decade of production use look
  identical to generation. JD-relevance filtering is the current stopgap; the
  fix is populating real per-skill depth, not more schema.

Resolved: the renderer service question (Go-native vs Python) — Python won,
see Architecture above.
