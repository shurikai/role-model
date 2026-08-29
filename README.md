# Role Model

![CI](https://github.com/shurikai/role-model/actions/workflows/ci.yml/badge.svg)

A career knowledge base that generates tailored, traceable resumes. Built in Go, backed by Postgres, reasoned over by Claude.

## What this is

Role Model is a personal tool for keeping a career history as structured data and generating resumes from it, instead of editing one resume document by hand for every application.

Most resume tools work from a pasted-in document and a prompt. Role Model works from a database. Your career history is stored as small "contribution" records, each one more detailed than any resume bullet would be. Contributions attach to positions, and positions attach to employers. When you apply somewhere, the system doesn't summarize a static document. It queries that data and builds a resume from the contributions that fit the role.

Resumes mislead by what they leave out and what they pick, not only by what they make up. So the hard problem here isn't getting the LLM to write good bullets. It's modelling a career richly enough that choosing the right ten bullets out of two hundred is a retrieval problem rather than a hallucination problem.

## Quickstart

Docker and an Anthropic API key, nothing else:

```bash
cp .env.example .env    # set ANTHROPIC_API_KEY and JWT_SECRET
docker compose up --build
```

Open <http://localhost:8080> and sign up. Migrations run on their own before the
API starts, and the app and API share one origin, so there is no CORS to set up.
`.env.example` ships `SIGNUP_ENABLED=true`; set it to `false` once your account
exists if anyone else can reach the instance. To try the pipeline on a sample
career first, run `make seed-sample` or `make seed-clinical`.

### From source

Go, Node, `uv`, Docker for Postgres, and `migrate` / `sqlc` / `psql` on your PATH:

```bash
make setup          # env files, database, migrations, npm install
make dev            # API, frontend, renderer together; Ctrl-C stops all three
```

The frontend runs on `:5173` and the API on `:8080`, so `CORS_ALLOWED_ORIGINS`
applies here and defaults to `http://localhost:5173`.

Full detail, including running more than one instance side by side, is in
[Getting started](#getting-started) below.

## How it works

Role Model runs a two-stage LLM pipeline on top of a relational schema:

**Stage 1 extracts JD signals.** Given a job description, one LLM call pulls out structured signals: seniority, required and preferred skills, the competency-level asks a posting states in prose, culture signals, and a screening summary in the posting's own words. These are stored as JSONB on the application, not re-derived on every use.

**Stage 2 generates the resume.** It takes the extracted signals plus a context assembled from the career database (employers, their positions, each position's contributions) and produces a resume in a fixed JSON schema. Every bullet records the contribution IDs it came from. Nothing lands in a resume without a traceable origin in the data.

Stage 2 is two calls, not one. The first selects and writes the bullets and skills against the JD signals, under a length budget set by the target seniority. The second writes the summary, and it sees only the bullets the first call selected. That split is deliberate. Give a summary the whole career to look at and it will claim things the bullets don't support, which is what happened before the split existed.

Between the two stages sits a **fit gate**, a deterministic scoring pass over capability coverage and stated preferences. The scoring is plain Go. The LLM only writes the narrative that explains scores it didn't produce. The gate separates a preference the posting doesn't mention from one it conflicts with, because collapsing the two turns "they didn't say" into "they're wrong for you."

That makes Role Model a bespoke retrieval-augmented system. SQL retrieval instead of a vector store, driven by the Stage 1 signals. It is deliberately not agentic. There is no autonomous loop deciding what to do next. Each stage is one deterministic LLM call with a fixed input shape and a fixed output schema, and a human reviews the result before it goes anywhere. Auditable and reviewable beats clever and autonomous.

## Why it's structured this way

**Contributions are atoms, not bullets.** Each one captures a piece of work in more detail than would ever fit on a resume. A resume bullet is a per-application view of an atom, not the atom itself. That is what makes tailoring possible without rewriting from scratch every time.

**Positions split by title inside long-tenure employers.** Seventeen years at one company isn't one job. It's several jobs that happened to share a building. Splitting on each title change keeps the level and scope of every role honest.

**Every generated bullet carries provenance.** A `contribution_ids` field on each bullet ties it back to the source data. If a bullet looks off, the question "where did that come from" has an answer.

**Career data enters through review, not through a form.** The main way in is
the career import. Paste a résumé, a CV, or free-form notes, and an LLM pass
turns it into staged drafts: employers, positions, contributions, skills,
preferences, education, credentials. You approve or reject each one. Nothing
reaches your record unreviewed. If a draft's parent is still pending, the
import refuses it by name instead of cascading, because resolving on the
reviewer's behalf is the one thing a review queue exists to prevent.

Versioned seed SQL is still supported, and it is how the author's own history
is kept. The files are idempotent, and corrections go in as new numbered files
rather than edits to old ones, so the history of what changed sits beside the
data. Both paths write the same tables.

**Single-user by default, self-hostable by design.** One person, one instance,
one database, one API key. Signup is closed unless you open it. There is no
billing and nothing shared with anyone. Every tenant-scoped table already
carries a `user_id`, so a multi-tenant path stays open without a redesign, but
that is a deployment decision and it hasn't been made.

So your career history and your Anthropic key never leave your machine, and
running this costs whatever your own key costs.

## Stack

- **Go** with `chi` for routing. Chosen for simplicity and ubiquity
- **PostgreSQL 16** via `pgx/v5` (native interface), with `sqlc` for type-safe generated queries and `golang-migrate` for schema migrations
- **Anthropic Go SDK** for both pipeline stages, with prompts stored as `text/template` files and compiled into the static binary via `go:embed`. A prompt's identity is the git blob hash of its content, recorded per generation, so any past resume resolves back to the exact prompt text that produced it
- **JSON Schema validation** (`santhosh-tekuri/jsonschema`) against a fixed resume schema, so generation output is structurally guaranteed before storage
- **Python** (FastAPI + `python-docx`, managed with `uv`) for the DOCX renderer only, which runs as a separate process rather than part of the service
- **React + TypeScript** frontend (Vite, TanStack Query, React Router), covering the auth shell and the application generation flow

## Getting started

### With Docker (recommended)

Requires Docker and an Anthropic API key. Nothing else. No Go, Node, `uv`,
`migrate` or `psql` on your PATH.

```bash
cp .env.example .env    # then set ANTHROPIC_API_KEY and JWT_SECRET
docker compose up --build
```

Then open <http://localhost:8080>.

That is the whole instance behind one URL. The web container serves the app
and proxies `/api/v1` to the API from the same origin, so there is no CORS to
configure. The address works whether you reach it as `localhost`, a LAN
address, or a hostname. Migrations run automatically before the API starts.

Two things the compose file will not let you skip. `JWT_SECRET` must be set.
An empty one signs every token with a zero-length key, so the server refuses
to start. `ANTHROPIC_API_KEY` must be yours. **You supply your own key and pay
for your own tokens; this project never ships one.**

Signup is closed by default outside development, because an open signup on a
reachable instance lets anyone create an account and spend that key. To create
your first account, start with `SIGNUP_ENABLED=true` in `.env`, sign up, then
set it back to `false` and `docker compose up -d` again.

The database is published on port 5433 for `psql` and the integration tests.
An instance exposed to a network should drop that line from
`docker-compose.yml`. The renderer and the API are deliberately not published
at all. The renderer in particular has no authentication and is meant to sit
behind the API on a private network.

**Compose reuses whatever database its project already has.** Data lives in a
named volume, so `docker compose up` attaches to the existing one rather than
starting empty. That is what you want for your own instance, and not what you
want when you meant to start fresh. A different project name gets its own
volume, and so its own database, migrated and empty:

```bash
docker compose -p rolemodel-demo up --build   # a clean instance, alongside yours
docker compose -p rolemodel-demo down -v      # and gone again, volume included
```

Set `WEB_PORT` in that project's environment if the default 8080 is taken. To
wipe the database an instance already has, run `down -v` on its own project.
The `-v` is what removes the volume. Without it, a rebuild comes back with the
same data.

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

`make dev` runs the three processes in one terminal. `make run`,
`make run-frontend` and `make run-renderer` start them individually. Here the
frontend is on `:5173` and the API on `:8080`, which are different origins, so
`CORS_ALLOWED_ORIGINS` matters on this path. It defaults to
`http://localhost:5173`.

Other useful targets: `make test` (Go unit tests), `make test-integration`
(needs a running database), `make test-renderer` (the renderer's pytest
suite), `make fmt` / `make fmt-check` (Go, TypeScript and Python together),
`make migrate-create`, `make db-reset`, `make db-down`.

### Your own career history

`make seed` loads career data from `$SEED_DIR`, which points at a **separate
private repo** checked out in place at `database/seed` and gitignored here.
That is one way in, and it is the author's. It is not the path a new user
takes. The one you want is the career import in the UI: paste a résumé, a CV,
or free-form notes, review what it extracted, and approve it. Nothing is
written to your record until you do.

There is no password reset in the UI yet. Until there is,
`make reset-password EMAIL=you@example.com` prompts for a new password and
writes the hash directly. It reads from stdin rather than an argument, so it
needs a real terminal. Set `NEWPASS` in the environment to run it
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

The DOCX renderer lives in `docx-renderer/` and is managed with `uv`. It's a standalone FastAPI service. The Go service calls it over HTTP and does not embed it.

```bash
cd docx-renderer
uv run uvicorn main:app --reload --port 8000
```

## Status

The path from a pasted job description to a downloaded `.docx` runs end to end
without leaving the browser, and the whole stack comes up with one command.

- **Backend.** Full CRUD for employers, positions, contributions, applications,
  tags and tag categories, education, credentials, projects, skills and
  preferences. JWT auth (signup/login/me, 24h token, no refresh), with signup
  closed by default outside development. Both LLM stages: JD signal extraction,
  and generation split into a body call and a summary call scoped to the
  bullets the body already selected. A deterministic fit gate over capability
  coverage and stated preferences, where the LLM writes the narrative and
  scores nothing. Two import paths: one stages contributions against positions
  you already have, the other stages the employers and positions too, which is
  what a new account needs.
- **Frontend.** Auth, a nav shell, the application flow (paste a JD, review the
  fit report, generate, download), and review queues for both import paths, on
  Vite + React + TypeScript + TanStack Query with Vitest coverage.
- **Renderer.** `docx-renderer/`, a Python service (FastAPI + `python-docx`)
  with a single `POST /render` endpoint taking the intermediate resume JSON and
  returning a `.docx`. It never touches the database, and it is not exposed
  outside the compose network. Layout is explicit and compact rather than built
  on Word heading styles, with widow/orphan protection so section and role
  headers don't strand at a page break.

Output is validated against `schema/resume.v2.json` before storage. A per-user
manifest controls the document's shape: which sections print, in what order,
under what heading. The seniority ladder and the depth scale are user-owned
rows rather than enums. So the pipeline works for a career that is not
software, and `database/sample-clinical/` is the proof: a nurse's career,
built through the import rather than written by hand.

A job discovery worker is designed but not started. It would scrape public ATS
feeds like Greenhouse and Ashby and fan out through Kafka to extraction and
notification consumers.

## Known gaps

What's missing:

- **No career-data browsing or editing in the UI.** Import review is the only
  way to see what was captured. Correcting a contribution afterwards means the
  API or raw SQL. Skills and preferences can be edited on the Profile screen.
  Employers, positions, and contributions cannot.
- **No password reset in the app.** `make reset-password` is the stopgap.
- **No review gate on extracted JD signals** before generation runs.
- **Rendered documents are not persisted.** The `.docx` is streamed back to
  the caller. A blob storage interface is the intended fix.
- **Depth is only half-used.** A posting that asks for expert Kafka no longer
  scores a novice Kafka as a clean match. But most postings state no depth, and
  there a one-off prototype and a decade of production use still score the
  same. Fixing that would rescore every stored application, so it's a decision
  rather than a bug.

## Documentation

- [`SECURITY.md`](./SECURITY.md) — what to do before exposing an instance, what
  is in place, and what deliberately is not
- [`CONTRIBUTING.md`](./CONTRIBUTING.md) — how to build and test it, and what
  will get a patch sent back
- [`CHANGELOG.md`](./CHANGELOG.md) — notable changes by release
- [`CLAUDE.md`](./CLAUDE.md) — the conventions document: stack, architecture,
  and the rules that hold, most of them written because the alternative was
  tried and cost something
- [`ROADMAP.md`](./ROADMAP.md) — the phase map and positioning statement;
  planned work is tracked in GitHub Issues, grouped by milestone (one per phase)

## License

Apache License 2.0. See [LICENSE](./LICENSE).
