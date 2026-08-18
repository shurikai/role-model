# Role Model — DOCX Renderer

Renders the intermediate resume JSON produced by [Role Model](../README.md)
into a `.docx`. FastAPI + `python-docx`, managed with `uv`.

This is a separate process, not part of the Go binary. The Go service reaches
it over HTTP through `internal/renderer.Client`.

## Why it's a separate service

Go's DOCX story is thin, and resume layout is fussy in ways that are much
cheaper to express against `python-docx`. Isolating it keeps document
formatting concerns out of the API service entirely — the renderer's whole
contract is one endpoint that takes JSON and returns a file.

**The renderer never touches the database.** The JSON document it receives is
self-contained, and it holds no credentials, no connection string, and no
notion of a user. Anything the document needs must already be in the document.

## API

A single endpoint:

```
POST /render
Content-Type: application/json
```

The request body is a resume document conforming to
[`schema/resume.v1.json`](../schema/resume.v1.json). The response is the
rendered `.docx` as
`application/vnd.openxmlformats-officedocument.wordprocessingml.document`,
with a `Content-Disposition` attachment filename of `resume.docx`.

The schema is the contract between the generation pipeline and this service.
`models.py` mirrors it as Pydantic models, so a malformed document fails
validation at the boundary with a 422 rather than producing a broken file.

## Layout

`renderer/docx_builder.py` builds the document explicitly rather than leaning
on Word's built-in heading styles, because those carry theme-dependent
formatting that renders inconsistently across Word versions and platforms.
Page geometry, spacing, and colors are declared as constants at the top of
that module.

Section headings and role-header lines set `keep_with_next` so a heading can't
strand at the foot of a page. Bullets are deliberately left free to break
across pages — a long bullet list that refuses to split wastes more space than
the orphan it prevents.

## Getting started

Requires Python 3.14 and `uv`.

```bash
uv sync                                          # install dependencies
uv run uvicorn main:app --reload --port 8000     # start the service
```

From the repository root, `make run-renderer` does the same thing, and
`make dev` starts the renderer alongside the API and the frontend.

The Go service finds it via `RENDERER_URL` (see `.env.example` at the root); it
defaults to `http://localhost:8000` in the example config, and the API logs a
warning at startup if it is unset.

### Trying it directly

The renderer is easy to exercise on its own, without the API or a database:

```bash
curl -X POST http://localhost:8000/render \
  -H 'Content-Type: application/json' \
  -d @../tests/fixtures/sample_resume.json \
  -o out.docx
```

`docs` at `http://localhost:8000/docs` gives the usual FastAPI interactive
schema browser.

## Layout of this directory

- `main.py` — FastAPI app and the `/render` endpoint
- `models.py` — Pydantic models mirroring `schema/resume.v1.json`
- `renderer/docx_builder.py` — the document builder and all layout constants

## Development

```bash
uv run pytest                 # run the tests
uv run ruff format .          # format
uv run ruff format --check .  # verify formatting
```

`make test-renderer` at the repository root runs the tests; `make fmt` runs the
formatter along with `gofmt` and Prettier.

The suite covers three things, against the shared fixtures in
`tests/fixtures/` rather than a private copy:

- **The schema contract** (`test_models.py`) — `models.py` mirrors
  `schema/resume.v1.json`, and nothing enforces that the two agree. These pin
  the invariants the pipeline relies on, including that a bullet cannot
  validate without at least one `contribution_ids` entry.
- **Document construction** (`test_docx_builder.py`) — that no employer,
  position, bullet, or skill is silently dropped, and that the layout
  invariants hold: no Word heading styles, `keep_with_next` on the header
  chains, bullets left free to break.
- **The HTTP contract** (`test_render_endpoint.py`) — status, media type,
  attachment filename, and a 422 rather than a 500 on a malformed document.
  `internal/renderer.Client` depends on all four.

Deliberately not asserted: the `.docx` bytes themselves. Binary comparison
against the tracked fixtures would fail on every incidental spacing change.
