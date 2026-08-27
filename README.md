# Role Model
![CI](https://github.com/shurikai/role-model/actions/workflows/ci.yml/badge.svg)

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

**Career data enters through review, not through a form.** The main path in is
the career import: paste a résumé, a CV, or free-form notes about what you did,
and an LLM pass turns it into staged *drafts* — employers, positions,
contributions, skills, preferences, education, credentials — which you approve
or reject one at a time. Nothing reaches your record unreviewed, and a draft
whose parent is still pending is refused by name rather than cascaded, because
resolving on the reviewer's behalf is the one thing a review queue exists to
prevent.

Versioned seed SQL is still a supported way in, and it is how the author's own
history is kept: idempotent files, corrections as new numbered files rather
than edits to old ones, so the record of what changed survives beside the data.
Both paths write the same tables.

**Single-user by default, self-hostable by design.** One person, one instance,
their own database and their own API key — signup is closed unless you open it,
and there is no billing, no tenant isolation to trust, and nothing shared with
anyone. Every tenant-scoped table already carries a `user_id`, so a multi-tenant
path stays open without a redesign, but that is a deployment decision rather
than a schema one and it has not been made.

What follows from that: your career history and your Anthropic key never leave
your machine, and the cost of running this is whatever your own key costs.

## Stack

- **Go** with `chi` for routing. Chosen for simplicity and ubiquity
- **PostgreSQL 16** via `pgx/v5` (native interface), with `sqlc` for type-safe generated queries and `golang-migrate` for schema migrations
- **Anthropic Go SDK** for both pipeline stages, with prompts stored as `text/template` files and compiled into the static binary via `go:embed`. A prompt's identity is the git blob hash of its content, recorded per generation, so any past resume resolves back to the exact prompt text that produced it
- **JSON Schema validation** (`santhosh-tekuri/jsonschema`) against a fixed resume schema, so generation output is structurally guaranteed before storage
- **Python** (FastAPI + `python-docx`, managed with `uv`), scoped narrowly to the DOCX renderer — a separate process, not the service itself
- **React + TypeScript** frontend (Vite, TanStack Query, React Router), covering the auth shell and the application generation flow

## Getting started

### With Docker (recommended)

Requires Docker and an Anthropic API key. Nothing else — no Go, Node, `uv`,
`migrate` or `psql` on your PATH.

```bash
cp .env.example .env    # then set ANTHROPIC_API_KEY and JWT_SECRET
docker compose up --build
```

Then open <http://localhost:8080>.

That is the whole instance behind one URL: the web container serves the app
and proxies `/api/v1` to the API from the same origin, so there is no CORS to
configure and the address works whether you reach it as `localhost`, a LAN
address, or a hostname. Migrations run automatically before the API starts.

Two things the compose file will not let you skip. `JWT_SECRET` must be set —
an empty one signs every token with a zero-length key, so the server refuses
to start — and `ANTHROPIC_API_KEY` must be yours. **You supply your own key
and pay for your own tokens; this project never ships one.**

Signup is closed by default outside development, because an open signup on a
reachable instance lets anyone create an account and spend that key. To create
your first account, start with `SIGNUP_ENABLED=true` in `.env`, sign up, then
set it back to `false` and `docker compose up -d` again.

The database is published on port 5433 for `psql` and the integration tests.
An instance exposed to a network should drop that line from
`docker-compose.yml`. The renderer and the API are deliberately not published
at all — the renderer in particular has no authentication and is meant to sit
behind the API on a private network.

To try the pipeline without a career of your own:

```bash
make seed-sample      # a backend engineer in freight logistics
make seed-clinical    # a nurse, built through the intake
```

### Without Docker

Requires Go, Node, Docker (for Postgres), `uv`, and `migrate`, `sqlc` and
`psql` on your PATH.

```bash
make setup      # .env files, database, migrations, npm install
                # then set ANTHROPIC_API_KEY and JWT_SECRET in .env
make seed-sample
make dev        # API, frontend and renderer together; Ctrl-C stops all three
```

`make dev` runs the three processes in one terminal; `make run`,
`make run-frontend` and `make run-renderer` start them individually. Here the
frontend is on `:5173` and the API on `:8080`, which *are* different origins —
so `CORS_ALLOWED_ORIGINS` matters on this path, and defaults to
`http://localhost:5173`.

Other useful targets: `make test` (Go unit tests), `make test-integration`
(needs a running database), `make test-renderer` (the renderer's pytest
suite), `make fmt` / `make fmt-check` (Go, TypeScript and Python together),
`make migrate-create`, `make db-reset`, `make db-down`.

### Your own career history

`make seed` loads career data from `$SEED_DIR`, which points at a **separate
private repo** checked out in place at `database/seed` and gitignored here.
That is one way in, and it is the author's; it is not the path a new user
takes. The one you want is the career import in the UI — paste a résumé, a CV
or free-form notes, review what it extracted, and approve it. Nothing is
written to your record until you do.

There is no password reset in the UI yet. Until there is,
`make reset-password EMAIL=you@example.com` prompts for a new password and
writes the hash directly. It reads from stdin rather than an argument, so it
needs a real terminal; set `NEWPASS` in the environment to run it
non-interactively.

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
[ ] Build out the career-data views (employer/position/contribution browsing and editing)
[ ] Weight technical fit scoring by skill proficiency and years (the schema and the data are both there; `internal/fitgate` reads names only)
[ ] Human review gate for extracted JD signals before generation runs

## License
Apache License 2.0 — see [LICENSE](./LICENSE).
