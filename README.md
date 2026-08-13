# Role Model

A career knowledge base that generates tailored, traceable resumes. Built in Go, backed by Postgres, reasoned over by Claude.

## What this is

Role Model is a personal tool for managing a career history as structured data and generating resumes from it, rather than hand-editing a single document into oblivion across dozens of applications.

Most resume tools work from a pasted-in document and a prompt. Role Model works from a database. Career history is stored as discrete, over-complete "contribution" atoms — richer and more specific than any single resume bullet would be — attached to positions, which are attached to employers. When it's time to apply somewhere, the system doesn't summarize a static document; it queries the underlying data and assembles a resume from the atoms that actually fit the role.

This matters because resumes lie by omission and by selection, not just by fabrication. The interesting problem isn't "make the LLM write good bullets" — it's "model a career richly enough that picking the right ten bullets out of two hundred is a retrieval problem, not a hallucination problem."

## How it works

Role Model runs a two-stage LLM pipeline on top of a relational schema:

**Stage 1 — JD signal extraction.** Given a job description, an LLM call extracts structured signals: seniority level, priority skills, domain vocabulary, the things that matter for matching. This is stored as JSONB on the application record, not re-derived every time.

**Stage 2 — Resume generation.** Given the extracted signals and a context assembled from the career database (employers, their positions, each position's contributions), the system generates a resume conforming to a fixed JSON schema. Every bullet traces back to the contribution ID(s) it was generated from — nothing appears in a resume that doesn't have a concrete, auditable origin in the underlying data.

Stage 2 is two calls, not one. The first selects and writes bullets and skills against the JD signals, under a length budget derived from the target seniority. The second writes the summary, and — this is the point — it only sees the bullets the first call already selected. A summary that can't see the whole career can't claim things the resume doesn't support, which was a real failure mode before the split.

Between Stage 1 and Stage 2 sits a **fit gate**: a deterministic scoring pass over technical coverage and stated preferences. The scoring is plain Go; the LLM only writes the prose narrative explaining scores it did not produce. The gate distinguishes a preference the posting merely doesn't mention from one it actively conflicts with, because conflating the two turns "they didn't say" into "they're wrong for you."

Architecturally, this makes Role Model a bespoke retrieval-augmented system: structured SQL retrieval in place of a vector store, with the retrieval step driven by the signals extracted in Stage 1. It is deliberately not agentic. There's no autonomous loop deciding what to do next. Each stage is a single, deterministic LLM call with a fixed input shape and a fixed output schema, and a human reviews the result before it goes anywhere. The thinking was that auditable and reviewable beats clever and autonomous.

## Why it's structured this way

**Contributions are atoms, not bullets.** Each one captures a piece of work in more detail than would ever fit on a resume — the resume bullet is a *projection* of the atom, generated per-application, not the atom itself. This is what makes tailoring possible without rewriting from scratch every time.

**Positions are split by title within long-tenure employers.** Seventeen years at one company isn't one job; it's several jobs that happened to share a building. Splitting on title change keeps the level and scope of each role honest.

**Every generated bullet carries provenance.** A `contribution_ids` field on each bullet ties it back to the source data. If a bullet looks off, the question "where did that come from" has an actual answer.

**Career data is seeded, not written through the API.** Write endpoints for career history were considered and deliberately skipped in favor of versioned, idempotent seed files. This isn't a CRUD app for editing a career — the career data changes slowly and deliberately, and seed files with upsert semantics are a better fit than a UI for that cadence.

**This is closed and single-user on purpose.** The schema carries a `user_id` on tenant-scoped tables, which would make a multi-tenant path possible without a redesign — but there's no near-term intention to license this to other users or build it as a product. It's a job-search tool first and a portfolio piece second. Both of those goals are served by depth and correctness, not by adding auth, billing, and multi-tenant guarantees nobody asked for.

## Stack

- **Go** with `chi` for routing. Chosen for simplicity and ubiquity
- **PostgreSQL 16** via `pgx/v5` (native interface), with `sqlc` for type-safe generated queries and `golang-migrate` for schema migrations
- **Anthropic Go SDK** for both pipeline stages, with prompts versioned as templates and included in the static executable via `go:embed`
- **JSON Schema validation** (`santhosh-tekuri/jsonschema`) against a fixed resume schema, so generation output is structurally guaranteed before storage
- **Python** (FastAPI + `python-docx`, managed with `uv`), scoped narrowly to the DOCX renderer — a separate process, not the service itself
- **React + TypeScript** frontend (Vite, TanStack Query, React Router), covering the auth shell and the application generation flow

## Getting started

Requires Go, Docker, `uv`, and `migrate`, `sqlc`, and `psql` on your `PATH`.

```bash
cp .env.example .env   # then fill in ANTHROPIC_API_KEY and adjust DATABASE_URL/SEED_DIR as needed

make db-up              # start Postgres in Docker
make migrate-up         # apply schema migrations
make sqlc               # generate query code from SQL
make seed               # load career history from $SEED_DIR (see note below)
                        # -- or `make seed-sample` for the bundled fictional dataset
make dev                # start the API, the frontend, and the renderer together
```

A full stack is three processes. `make dev` runs all of them and stops the whole set on Ctrl-C; `make run`, `make run-frontend`, and `make run-renderer` start them individually if you'd rather have separate terminals.

Other useful targets: `make test` (unit tests), `make test-integration` (requires a running database), `make migrate-create` (scaffold a new migration), `make db-reset` (drop and recreate the database volume), `make db-down` (stop the database).

There is no password reset flow in the UI yet. Until there is, `make reset-password EMAIL=you@example.com` prompts for a new password and writes the hash directly. It reads the password from stdin rather than an argument, so it needs a real terminal; set `NEWPASS` in the environment to run it non-interactively.

Career history itself — employers, positions, contributions — lives in a separate private repo as a set of versioned, idempotent SQL seed files, pointed to by `SEED_DIR`. Check it out in place at `database/seed` (gitignored here, so it is never tracked by this repo) and leave `SEED_DIR="./database/seed"`; any other path works equally well. This is intentional (see "Career data is seeded, not written through the API" above): the whole point is that this seeds *your* career, not a demo one.

To try the pipeline without a career-history seed set of your own, `make seed-sample` loads a bundled fictional dataset from `database/sample/` — a backend/platform engineer in freight logistics, with three employers, six positions, 35 contributions, varied skill depth, and a full preference set. Three paired JD fixtures in `tests/fixtures/` exercise a strong match, a poor match, and the anti-pattern hard gate. See [`database/sample/README.md`](database/sample/README.md). It is a separate target from `make seed` on purpose, so an absent-minded invocation can't mix invented employers into real career history.

### Frontend

The frontend lives in `frontend/` and talks to the API above. See `frontend/README.md` for details.

```bash
cd frontend
npm install
npm run dev     # start the Vite dev server
npm test        # run the Vitest suite
```

### Renderer

The DOCX renderer lives in `docx-renderer/` and is managed with `uv`. It's a standalone FastAPI service; the Go service calls it over HTTP and does not embed it.

```bash
cd docx-renderer
uv run uvicorn main:app --reload --port 8000
```

## Status

Both pipeline stages work end to end against the real API, output is validated against `schema/resume.v1.json` before storage, and the surrounding CRUD surface is built out:

- **Backend:** full CRUD for employers, positions, and contributions; application CRUD; tags and tag categories; education, credentials, and projects; JWT auth (signup/login/me, 24h token, no refresh); JD signal extraction and resume generation (Stages 1–2); Stage 0 import (LLM-assisted extract + enrich, with human approve/reject) for pulling new contributions out of raw text; a fit-gate scoring pass, backed by skills and preferences tables, that flags weak job/candidate matches before spending a generation call; config-driven CORS.
- **Frontend:** the auth shell (login, signup, session persistence, 401 handling) plus the application generation flow — paste a JD, review the fit assessment, generate, download the `.docx` — on Vite + React + TypeScript + TanStack Query, with test coverage (Vitest).
- **Renderer:** `docx-renderer/`, a Python service (FastAPI + `python-docx`) with a single `POST /render` endpoint taking the intermediate resume JSON and returning a `.docx`. It never touches the database. Layout is explicit and compact rather than built on Word heading styles, with widow/orphan protection so section and role headers don't strand at a page break.

The path from a pasted job description to a downloaded `.docx` now runs end to end without leaving the UI.

A job discovery worker (scraping public ATS feeds like Greenhouse and Ashby, fanning out via Kafka to extraction and notification consumers) is designed but not started.

This README will grow as the system does.

## Open Questions
- Blob storage interface for rendered documents (local disk now, S3 later) — rendered files are currently returned to the caller, not persisted
- Evaluation strategy for prompt quality across versions
- Skills carry proficiency and years-of-experience, and both are populated with real values — but the fit scorer reads skills as a flat list of names and never sees either column, so a one-off prototype and a decade of production use score identically. The fix is plumbing the depth through to `internal/fitgate`, not more seeding

## TODO
[ ] Restore `applied_on` date parsing on the application update endpoint
[ ] Add sample migrations with dummy test data (for onboarding without real career data)
[ ] Build out the career-data views (employer/position/contribution browsing and editing)
[ ] Weight technical fit scoring by skill proficiency and years (the schema and the data are both there; `internal/fitgate` reads names only)
[ ] Human review gate for extracted JD signals before generation runs
