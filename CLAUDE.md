# Role Model

**Repository:** https://github.com/shurikai/role-model

A self-hostable, single-user service for AI-powered resume generation.
Stores detailed career history as structured data and generates tailored,
versioned resumes per job application using an LLM.

Designed with a clear path to multi-tenant deployment without requiring
a schema redesign.

## Status
Early development. Schema, JSON contract, and project skeleton are defined. Business logic not yet implemented.

## Stack
- **Language:** Go
- **Router:** chi
- **Database:** PostgreSQL via pgx/v5 (native interface, not database/sql)
- **Query generation:** sqlc
- **Migrations:** golang-migrate
- **LLM:** Anthropic API via official Go SDK
- **Prompt storage:** /prompts directory, embedded via go:embed
- **Prompt templating:** text/template
- **JSON schema validation:** santhosh-tekuri/jsonschema

## Architecture

### Two-prompt LLM pipeline
1. **JD signal extraction** — takes raw job description text, returns structured
   jd_signals JSON (priority skills, seniority, domain vocabulary)
2. **Resume generation** — takes jd_signals + contributions + identity + prior
   feedback, returns a complete intermediate resume JSON document

### Intermediate resume JSON
Generation produces a structured JSON document (see /schema/resume.v1.json)
that is the contract between the generation pipeline and the renderer.
The renderer never touches the database. The JSON document is self-contained.

### Renderer
Not yet built. Will be a separate concern, likely a small Python service
using python-docx, communicating with the Go service over HTTP.
The Go service owns generation; the renderer owns document output.

### Prompt management
Prompts are versioned files in /prompts, embedded into the binary at compile
time via go:embed. Prompt version is recorded in resume_versions.generation_params
so every generated resume can be traced back to the exact prompt that produced it.

## Project Structure
/cmd/server          — main entry point
/internal/api        — HTTP handlers
/internal/db         — sqlc generated code
/internal/generation — LLM pipeline (signal extraction + resume generation)
/internal/renderer   — intermediate JSON -> output format
/migrations          — golang-migrate SQL migration files
/prompts             — LLM prompt template files
/schema              — JSON schema documents
/experiments         — Python scripts for prompt development (not part of build)

## Key Files
- /CLAUDE.md                 — project instructions and conventions (this file)
- /schema/resume.v1.json     — intermediate resume JSON schema (source of truth)
- /migrations/               — database migrations (source of truth for schema)
- /prompts/                  — LLM prompt templates

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

## API Design
- REST
- JSON request/response
- Structured error responses, not raw strings
- Environment-based config, no hardcoded values
- Auth is stubbed for single-user now, designed for JWT-based auth later

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
- Invent prompt improvements — prompts live in /prompts and are versioned explicitly

## Open Questions
- Blob storage interface for rendered documents (local disk now, S3 later)
- Auth implementation (stub for single-user, JWT for multi-user)
- Renderer service: Go-native vs Python/python-docx (deferred)
- Evaluation strategy for prompt quality across versions (deferred)
