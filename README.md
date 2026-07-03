# Role Model

A career knowledge base that generates tailored, traceable resumes. Built in Go, backed by Postgres, reasoned over by Claude.

## What this is

Role Model is a personal tool for managing a career history as structured data and generating resumes from it, rather than hand-editing a single document into oblivion across dozens of applications.

Most resume tools work from a pasted-in document and a prompt. Role Model works from a database. Career history is stored as discrete, over-complete "contribution" atoms — richer and more specific than any single resume bullet would be — attached to positions, which are attached to employers. When it's time to apply somewhere, the system doesn't summarize a static document; it queries the underlying data and assembles a resume from the atoms that actually fit the role.

This matters because resumes lie by omission and by selection, not just by fabrication. The interesting problem isn't "make the LLM write good bullets" — it's "model a career richly enough that picking the right ten bullets out of two hundred is a retrieval problem, not a hallucination problem."

## How it works

Role Model runs a two-stage LLM pipeline on top of a relational schema:

**Stage 1 — JD signal extraction.** Given a job description, an LLM call extracts structured signals: seniority level, priority skills, domain vocabulary, the things that matter for matching. This is stored as JSONB on the application record, not re-derived every time.

**Stage 2 — Resume generation.** Given the extracted signals and a context assembled from the career database (employers, their positions, each position's contributions), an LLM call generates a resume conforming to a fixed JSON schema. Every bullet traces back to the contribution ID(s) it was generated from — nothing appears in a resume that doesn't have a concrete, auditable origin in the underlying data.

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
- **Python**, scoped narrowly to prompt experimentation and a planned DOCX renderer — not the service itself
- **React + TypeScript** frontend (Vite, TanStack Query, React Router) for the auth shell, with more views to come as the backend surface grows

## Getting started

Requires Go, Docker, and `migrate`, `sqlc`, and `psql` on your `PATH`.

```bash
cp .env.example .env   # then fill in ANTHROPIC_API_KEY and adjust DATABASE_URL/SEED_DIR as needed

make db-up              # start Postgres in Docker
make migrate-up         # apply schema migrations
make sqlc               # generate query code from SQL
make seed               # load career history from $SEED_DIR (see note below)
make run                # start the API server
```

Other useful targets: `make test` (unit tests), `make test-integration` (requires a running database), `make migrate-create` (scaffold a new migration), `make db-reset` (drop and recreate the database volume), `make db-down` (stop the database).

Career history itself — employers, positions, contributions — lives outside this repo as a set of versioned, idempotent SQL seed files, pointed to by `SEED_DIR`. This is intentional (see "Career data is seeded, not written through the API" above); there's no bundled sample data, since the whole point is that this seeds *your* career, not a demo one.

### Frontend

The frontend (auth shell only, for now) lives in `frontend/` and talks to the API above. See `frontend/README.md` for details.

```bash
cd frontend
npm install
npm run dev     # start the Vite dev server
npm test        # run the Vitest suite
```

## Status

Both pipeline stages work end to end against the real API, output is validated against `schema/resume.v1.json` before storage, and the surrounding CRUD surface is built out:

- **Backend:** full CRUD for employers, positions, and contributions; application CRUD; JWT auth (signup/login/me, 24h token, no refresh); JD signal extraction and resume generation (Stages 1–2); Stage 0 import (LLM-assisted extract + enrich, with human approve/reject) for pulling new contributions out of raw text; a fit-gate scoring pass that flags weak job/candidate matches before spending a generation call; config-driven CORS.
- **Frontend:** an auth shell (login, signup, session persistence, 401 handling) on Vite + React + TypeScript + TanStack Query, with test coverage (Vitest).
- `internal/renderer` was removed as dead code — no consumer existed yet. Resume rendering (DOCX/PDF) is designed but not started; see Open Questions.

A job discovery worker (scraping public ATS feeds like Greenhouse and Ashby, fanning out via Kafka to extraction and notification consumers) is designed but parked until the renderer ships.

This README will grow as the system does.

## Open Questions
- Blob storage interface for rendered documents (local disk now, S3 later)
- Renderer service: Go-native vs. Python/python-docx — deferred deliberately; no interface has been pre-defined ahead of that session
- Evaluation strategy for prompt quality across versions

## TODO
[ ] Restore `applied_on` date parsing on the application update endpoint
[ ] Add sample migrations with dummy test data (for onboarding without real career data)
[ ] Scaffold the document renderer (DOCX, possibly PDF)
[ ] Build out frontend beyond the auth shell (employer/position/contribution/application views)
