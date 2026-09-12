# Role Model — Onboarding Agent

A conversational, tool-calling interview (#117) for populating career data by
interview rather than by pasting a document. LangGraph + FastAPI, managed
with `uv`. Parallel to Stage 0 (`internal/stage0`), not a replacement — Stage
0 handles "paste a resume"; this handles the person who has no clean document
to paste, or whose career doesn't decompose into pre-written bullets. Both
paths write the same tables, through the same REST endpoints.

This is a separate process, not part of the Go binary. The Go service reaches
it over HTTP through `internal/onboarding.Client`.

## Why it's a separate service, and why it's internal-only

LangGraph is Python; the rest of this backend is Go, and the conservative
Go-dependency rule in the repository root `CLAUDE.md` doesn't extend to a
service with its own `pyproject.toml` any more than it does to
`docx-renderer/`.

Unlike the renderer, this service DOES need to write data on the user's
behalf — but it never touches the database and never gets its own
credentials. The topology is:

```
Browser --(JWT)--> Go API (RequireAuth, as always)
                      --(relayed JWT, private network)--> onboarding-agent
                                                              --(same JWT)--> Go API's own REST endpoints
```

The Go API is the only public origin; this service is reachable only on the
compose network, the same security posture `docx-renderer` already has (see
its README). The one difference is that this service DOES act on the user's
behalf, by relaying the same bearer token the browser sent — it never has its
own credential, never verifies the token itself (an invalid one simply fails
the same way at the Go API it calls back into), and never bypasses the REST
API to touch Postgres directly.

**The token is never persisted.** LangGraph checkpoints the interview's state
to SQLite so a session survives a container restart, and a live credential
sitting in that file would be a problem. The token is threaded through
`config["configurable"]["token"]` on every call instead of through graph
state — see `agent/state.py`.

## API

A single endpoint:

```
POST /turns
Content-Type: application/json

{"session_id": null, "message": null, "token": "<the user's own JWT>"}
```

`session_id` is `null` on the first call of a new interview; the response's
`session_id` is then sent back on every subsequent turn along with the
person's `message`, resuming the LangGraph checkpoint from where it paused.
Response:

```
{"session_id": "...", "reply": "...", "done": false}
```

`done` is `true` only once the graph has run to completion with nothing left
to ask — derived from whether LangGraph's own `__interrupt__` marker is
present in the result, not tracked separately.

## The graph

`agent/graph.py` — employer → position → contributions (looped) → tag
candidates extracted from each contribution and confirmed one at a time →
proficiency/years asked only for tags that read as durable skills. See the
module docstring for why each loop is its own node with one `interrupt()`
call, rather than a Python loop with several.

**Every skill claim links to a supporting contribution.** A tag is only ever
attached (`agent/tools.py:attach_tag_to_contribution`) to the contribution
that was just recorded — never created floating free — which is what
`v_skill_provenance` (already in the schema) derives skill evidence from.
Nothing is written on the strength of the model's guess alone: `extract_tags`
only ever proposes candidates, and `confirm_next_tag` asks before anything is
attached.

`agent/tools.py` is the only thing that talks to the Go API, and every
function in it is an ordinary authenticated REST call — the same one a human
or the frontend could make.

## Getting started

Requires Python 3.14 and `uv`.

```bash
uv sync
uv run uvicorn main:app --reload --port 8100
```

From the repository root, `make run-onboarding` does the same thing, and
`make dev` starts this service alongside the API, the renderer, and the
frontend. The Go service finds it via `ONBOARDING_AGENT_URL` (see
`.env.example` at the root).

## Layout of this directory

- `main.py` — FastAPI app and the `/turns` endpoint
- `agent/state.py` — the graph's `InterviewState`
- `agent/graph.py` — the interview graph
- `agent/tools.py` — the Go REST API client
- `agent/tags.py` — the one LLM call (tag-candidate extraction from a
  contribution), isolated so the rest of the graph is testable without it

## Development

```bash
uv run pytest                 # run the tests
uv run ruff format .          # format
uv run ruff format --check .  # verify formatting
uv run ruff check .           # lint
```

`make test-onboarding` at the repository root runs the tests.

The graph's node/routing logic is tested against an in-memory checkpointer, a
fake Go API (`tests/fake_api.py`) standing in for `httpx.AsyncClient`, and a
fake `StructuredModel` returning a canned tag list — no real Anthropic or Go
API calls happen in the suite. `test_turns_endpoint.py` exercises `/turns`
end to end through FastAPI's `TestClient` against that same set of fakes.
