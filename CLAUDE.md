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
   structured jd_signals JSON: required/preferred skills, core competencies,
   seniority, domain, work type, culture signals, and a screening summary.

   **A JD's requirements arrive in two shapes, and both are extracted.**
   `required_skills`/`preferred_skills` hold named technology. `core_competencies`
   holds the capability-level asks a posting states in prose — "decomposing a
   legacy service", "production ownership of services", "setting technical
   direction". Senior and staff postings routinely name no technology at all,
   which left both skill lists correctly but uselessly empty and degraded every
   consumer at once and silently: the 2a requirement checklist rendered
   "(none listed)" twice, disabling the prompt's entire skill-relevance
   apparatus so it fell back to emitting the whole tag inventory, while
   `ScoreTechnicalFit` reported a vacuous 100 against nothing.

   The two lists stay separate because they are **satisfied** differently. A
   required skill can be answered by a Skills entry; a competency can only be
   evidenced by a bullet. Never let a competency into the Skills list — a
   resume listing "setting technical direction" among its technologies reads
   as padding and displaces a real skill.

   `core_competencies` is deliberately **not** in the document projection. Per
   the intermediate-JSON rule below, adding a field to `JDSignals` must not
   change what the document emits.
2. **Resume generation (Stage 2)** — split into two calls:
   - **2a body** (`resume_body.tmpl`) — selects and writes bullets and
     skills against jd_signals, under a seniority-informed length budget
     and framing guidance

     **Seniority drives two levers, and they are siblings on purpose.**
     `buildLengthBudget` sets how much gets written; `buildFramingGuidance`
     sets what altitude it is written at. Length was for a long time the only
     one, so a staff posting got more bullets of the same altitude rather than
     bullets pitched at the level it was hiring for — every other rule in the
     prompt pushes toward implementation specificity. Add a third lever to
     that pair, not somewhere else.

     Staff framing adds ownership and scope **on top of** the evidence, never
     in place of it. Trading the metric for the claim is the failure mode, not
     the goal: the number is what makes the ownership claim believable, and a
     broad claim with nothing behind it is what a skeptical reader discounts.
   - **2b summary** (`resume_summary.tmpl`) — writes the summary scoped to
     the bullets 2a already selected, so it cannot assert unsupported claims

   Facts that both calls would otherwise decide independently (e.g. the header
   title) are threaded through as explicit inputs rather than re-derived. This
   is the established pattern for cross-call consistency — follow it.

Between the two calls, `reconcileSkills` enforces the bullet/skills invariant
2a states but does not reliably honour: a claimed skill that an emitted bullet
names must appear in the Skills section. It runs before 2b so the summary is
written against the final Skills list. Three rules hold it together:

- **Re-add only, never drop.** A skill can be legitimately claimed without a
  dedicated bullet; dropping on that basis would delete a JD-relevant skill for
  want of room in the bullet budget.
- **Only claimed skills are eligible.** A bullet naming WAGO or NGTS must not
  manufacture a skill with no `skills` row behind it.
- **Whole-word matching, never substring** — the same rule the fit-gate matcher
  documents, for the same reason. Substring matching makes "Go" a hit inside
  "Golang" and "Java" a hit inside "JavaScript". The boundary test is
  "not alphanumeric" rather than a regexp `\b`, because real skill names carry
  punctuation (`C++`, `C#`, `.NET`, `CI/CD`) that `\b` breaks on.

Category order is preserved on rewrite. The renderer prints categories in
document order, so decoding to a plain map would silently re-alphabetize the
resume's Skills section as a side effect of adding one entry.

Both calls are recorded separately in generation_params for per-call
traceability.

### Fit gate
`internal/fitgate` runs a deterministic scoring pass before generation:
technical coverage plus preference fit, scored in Go. The LLM only writes
prose narrative from those scores — it interprets, it does not score.
Unmet preferences (JD simply doesn't mention something) and genuine conflicts
(JD involves something actively unwanted) are tracked as separate lists and
must stay that way; collapsing them produces false conflict language.

The two axes are orthogonal on purpose: technical score measures *capability*,
preference score measures *desire*. A role you could do and would hate should
read as high technical / low preference, not as one muddled number. Do not
introduce a blended score.

**A technical score can be absent, and absent is not zero and not 100.**
`ScoreTechnicalFit` returns a `TechnicalFit` whose `Scored` field is false when
the JD stated no technical requirements at all. It used to return a bare 100
there — a perfect score with no matches and no evidence, which the narrative
then wrote confident coverage prose around. "This profile answers none of the
requirements" and "this JD stated no requirements to answer" are opposite
findings and must not share a representation; that is why it is a struct field
rather than a sentinel value. When `Scored` is false the report stores SQL NULL
(the UI already renders that as "—") and the narrative input omits the score
entirely, which the prompt reads as "nothing was assessed". An empty
`technical_gaps` in that state means nothing was checked, not that there are no
gaps.

**Preferences carry severity and gate behavior separately.** `sentiment` is
`positive|negative`, `weight` is NOT NULL, and `is_hard_gate` marks the rows
that disqualify. A hard exclude is a heavy negative that also gates — there is
no `hard_exclude` sentiment (migration 011 removed it).

Hard-gate rows are deliberately **not** terms in the normalized average. A
matched gate subtracts its weight as raw points and then caps the score at
`hardGateCeiling`. Two rules that are easy to get wrong and are load-bearing:

- Feeding gates into `earned`/`possible` would let a profile full of unmatched
  excludes inflate every clean JD toward 100. Keep them out of the average.
- The ceiling is `min()`, an upper bound. Setting the score *to* the ceiling
  on a match would raise a JD that already scored below it.
- The empty-average short-circuit governs only the average. Penalty and
  ceiling still apply on top, or a gates-only profile scores 100 on a JD that
  trips one — the original bug, relocated.

Gating does **not** block. A tripped JD is still scored, still narrated, still
generated; the trip is priced into the score rather than living in a boolean.

**Technical matching runs in three layers, strongest first**, and the layer
that won is recorded on every match as `kind` (`direct` | `alias` | `category`):

1. **direct** — the JD term against the skill's own name. This is the only
   layer that keeps the raw-substring direction, so a JD asking for "SQL" is
   answered by "PostgreSQL".
2. **alias** — against `tags.aliases`. That column had been populated since
   migration 001 and read by nothing, so "Golang" scored a gap against a
   stored "Go" and "RESTful APIs" against "REST".
3. **category** — against the tag's category name and `tag_categories.aliases`
   (migration 012). This is what lets a competency-worded JD reach a
   technology-worded profile: "CI/CD" is answered by Jenkins and GitHub
   Actions, "observability" by Splunk and Dynatrace. A JD that names no
   concrete technology at all — common at staff level — otherwise scores 0
   with every requirement reported as a gap.

Two rules hold this together:

- **Aliases and category vocabulary are whole-word matched, never substring.**
  Only a skill's own name keeps the substring direction. A category alias is a
  sentence fragment, and substring-matching one made a JD requiring "RAG" match
  the Testing category — "rag" sits inside "test cove*rag*e" — offering JUnit
  as evidence of retrieval-augmented generation.
- **A category alias must name a capability, not a technology.** Putting
  "kafka" on Protocols & Messaging would grant the whole category for one tool.
  The converse also bites: bare "frameworks" is deliberately not an alias of
  Frameworks & Libraries, because "auth/authz frameworks" and "evaluation
  frameworks" would both claim credit for React.

Every match carries `evidence` — the specific skills behind it — so the
narrative cites what the person actually has instead of asserting a score, and
so a remaining gap is trustworthy. Gaps previously conflated "named
differently" with "does not have it".

**One matcher.** `prefFieldsFor` routes every preference by `preference_type`;
there is no second matcher for the gate, and `anti_pattern` is the only branch
that reads `required_skills`. The previous split (a broad `signalFields` for
scoring, a routed `gateFieldsFor` for the gate) is what hid #49: scoring never
saw the skills arrays, so a technology-shaped negative could not fire and,
because an unmatched negative earns its weight, paid out a bonus instead.

Conditional preferences are modelled by **decomposing the root cause into its
own weighted row**, not by a dependency edge. "C# is only bad because of the
Microsoft ecosystem" is two rows — a gate on the ecosystem, a moderate
negative on the language — and additive weights produce the conditional
behavior on their own. Do not add a parent/implies relation to `preferences`.

### Intermediate resume JSON
Generation produces a structured JSON document (see /schema/resume.v1.json)
that is the contract between the generation pipeline and the renderer.
The renderer never touches the database. The JSON document is self-contained.

**Nothing is copied into the document verbatim from an evolving upstream
type.** `meta.jd_signals` is a deliberate projection — `documentJDSignals` in
internal/generation, mirroring `$defs.jd_signals` field for field — not the
stored `jd_signals` blob. The schema sets `additionalProperties: false`, so
assigning the blob straight through coupled a strict contract to a type owned
by extraction, and it broke in both directions at once: 15 stored applications
carried the deprecated `priority_skills`/`domain_vocabulary` and 5 carried
`screening_summary`, none of them declared. 20 of 31 applications could not
generate at all, each failing validation on a field the document never needed.

The rule that follows: **adding a field to `JDSignals` must not change what the
document emits.** If the document should carry something new, the schema
declares it first and the projection follows — never the reverse. The same
reasoning applies to any future blob the document embeds.

`screening_summary` is deliberately absent from the document. It is screening
data (location, travel, clearance, comp), not resume content; the renderer has
no use for it, and it is already persisted on `fit_reports.screening_summary`.

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
Prompts live in /internal/generation/prompts, embedded into the binary at
compile time via go:embed.

**Prompt filenames carry no version number.** A prompt's identity is the git
blob hash of its content, computed from the embedded bytes by
`promptFingerprint` and recorded in `resume_versions.generation_params`. This
is the same hash git computes for the same bytes, so a recorded blob resolves
directly against the repository:

```
git cat-file -p <blob>          # the exact prompt text used
git log --find-object=<blob>    # the commit that introduced it, and why
git diff <blobA> <blobB>        # what changed between two generations
```

A prompt's *history* is `git log` on its path; its *rationale* is the commit
message plus the `{{/* */}}` header in the file.

Rules:
- **Never put a version number in a prompt filename or in a Go constant.** The
  scheme exists because a filename and a constant are two sources of truth for
  one fact, and they drifted — resumes were recorded against a prompt file that
  did not exist. Do not reintroduce that.
- **Commit prompt changes before generating anything you need to trace.** An
  uncommitted edit still hashes correctly and stably, but the blob exists in no
  commit and cannot be recovered later. `make check-prompts` warns; it runs
  automatically before `make run` and `make dev`. It warns rather than blocks
  on purpose — edit-and-regenerate is the normal tuning loop.
- Template `{{/* */}}` headers must end with the `-}}` trim marker, or the
  rendered prompt gains a leading newline. `TestPromptCommentsDoNotLeak` guards
  both the leak and the newline.
- `pipelineVersion` in prompts.go is separate and still hand-maintained. It
  names the *call sequence* (currently the 2a/2b split), which no individual
  file's content captures. Bump it when the shape of the pipeline changes, not
  when prompt text changes.
- `schema/resume.v1.json` requires a `prompt_version` field in the document
  `meta` block and sets `additionalProperties: false`, so the portable document
  carries `pipelineVersion` only. Per-prompt hashes live in generation_params
  on the DB row. Putting them in the document would require a schema v2 and a
  matching change to the renderer's Pydantic models.

## Project Structure
/cmd/server                      — main entry point
/cmd/resetpw                     — CLI to reset a user's password (no UI flow yet)
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
/database/seed                   — real career seed SQL; a separate private git
                                   repo checked out in place, gitignored here
/database/sample                 — fictional sample dataset, tracked here;
                                   loaded by `make seed-sample`, never `make seed`
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

## Formatting
Each language has one pinned formatter, and `make fmt` runs all three
(`make fmt-check` verifies without writing):

- **Go** — `gofmt`, from the toolchain
- **TypeScript** — Prettier, pinned in `frontend/package.json` with
  `.prettierrc`; also `npm run format` / `format:check`
- **Python** — `ruff format`, pinned in `docx-renderer`'s dev group with
  `[tool.ruff]` in pyproject.toml

Prettier is not always idempotent — a first `--write` pass can emit output that
a second pass reformats (it happened on a `vi.fn().mockResolvedValue()` chain).
Run it to convergence; do not assume one `--write` satisfies a later `--check`.

**SQL is deliberately not formatted.** Migrations are applied history that must
not churn, and the sqlc query files carry load-bearing `-- name: Foo :one`
directives that comment-reflowing formatters can silently break. Do not add
pg_format or sqlfluff.

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
- Invent prompt improvements — prompts live in /internal/generation/prompts
  (/prompts holds per-session task specs, not prompts)
- Add a version number to a prompt filename or a prompt version constant — see
  Prompt management above; content hashing replaced both
- Open new issues unprompted during a session focused on something else. If you
  notice unrelated work that should be tracked, mention it and let the human decide.
- Use the `claude-code-action` GitHub App or any webhook-triggered automation.
  All `gh` usage is interactive, inside a human-initiated session only.

## Open Questions
- Blob storage interface for rendered documents (local disk now, S3 later).
  Rendered .docx bytes are currently returned to the caller, not persisted.
- Evaluation strategy for prompt quality across versions (deferred)
- Skill depth signal — **the gap is in the code, not the data.** The schema
  supports depth (`skills.proficiency`, `skills.years_experience`, and
  `v_skill_provenance` deriving skill→contribution links from
  `contribution_tags`), and the data now carries it: migration 008's uniform
  `proficient` / `NULL` backfill was curated afterward in seed files 016/018/019,
  so the table holds a real spread of novice/proficient/expert with
  `years_experience` populated on most rows.

  **Generation now reads it; the fit gate still does not.** `assembleSkills`
  (`internal/generation/assemble.go`) selects the claimed skills with
  proficiency and years via `ListActiveSkillProfileByUser` and passes them to
  2a as `<skills>`, which filters on relevance and depth together and may
  annotate a few deep, central skills as "Java (25 yrs)". That block is also
  now the **only** source for the resume's Skills section — it used to be
  built from contribution tags, which are vocabulary rather than claims, and
  that is how JavaScript reached a rendered resume without a `skills` row
  behind it.

  `internal/fitgate` never sees any of it. `ListActiveSkillMatchTermsByUser`
  (`internal/db/queries/skills.sql`) selects name, aliases, and category —
  but not proficiency or years — and `ScoreTechnicalFit` takes `[]SkillTerm`
  built from exactly those columns. Scoring is therefore still pure
  presence/absence: a matched required skill is worth 2 points and a matched
  preferred skill 1, whether it represents twenty years or a weekend. A one-off
  prototype and a decade of production use still look identical to scoring, but
  because the columns are dropped at the query layer, not because they are
  empty. `ListActiveSkillProfileByUser` already selects exactly what is
  needed; threading proficiency and years through to the scorer is the fix.

  This is also why a category match earns full credit today (see the matching
  section above). Weighting a match by the depth behind it is the same missing
  signal, and belongs with this work rather than as a constant bolted onto the
  matcher.

Resolved: the renderer service question (Go-native vs Python) — Python won,
see Architecture above.
