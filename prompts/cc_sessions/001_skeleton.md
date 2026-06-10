Project: Role Model — a self-hostable Go REST API for AI-powered resume generation.
Repository: https://github.com/shurikai/role-model

Create the project skeleton. Do not implement any business logic. Stubs and
empty placeholders only.

## Stack
- Go (latest stable)
- chi router
- pgx/v5 (native interface, not database/sql)
- sqlc for query generation
- golang-migrate for migrations
- Anthropic Go SDK (anthropic-sdk-go)
- santhosh-tekuri/jsonschema for JSON schema validation
- text/template for prompt templating

## Directory structure to create
/cmd/server/main.go
/internal/api/router.go
/internal/api/handlers/health.go
/internal/db/db.go
/internal/generation/extractor.go
/internal/generation/generator.go
/internal/renderer/renderer.go
/internal/config/config.go
/migrations/                        — directory only, files already exist
/prompts/jd_extraction.v1.tmpl
/prompts/resume_generation.v1.tmpl
/prompts/cc_sessions/               — directory only
/schema/resume.v1.json              — already exists, do not touch
.env.example
.gitignore
CLAUDE.md
go.mod

## Specific requirements

### config/config.go
Typed Config struct loaded from environment variables at startup. Fields:
- DatabaseURL string
- AnthropicAPIKey string
- Port string (default "8080")
- Environment string (default "development")

### cmd/server/main.go
- Load config
- Establish pgx connection pool
- Wire chi router
- Start HTTP server
- Graceful shutdown on SIGINT/SIGTERM

### internal/api/router.go
- Wire chi router
- Mount /health endpoint
- Structured for adding route groups later (e.g. /api/v1/...)

### internal/api/handlers/health.go
- GET /health returns {"status": "ok"} with 200

### internal/db/db.go
- pgx connection pool initialization
- Accept config, return pool and error

### internal/generation/extractor.go
- Stub for JD signal extraction
- Struct for input (raw JD text) and output (JDSignals)

### internal/generation/generator.go
- Stub for resume generation
- Struct for input (JDSignals, contributions, identity, etc.) and output
  (structured resume JSON)

### internal/renderer/renderer.go
- Interface definition only: Render(resumeJSON []byte) ([]byte, error)
- No implementation

### Prompt template files
Populate /prompts/jd_extraction.v1.tmpl and /prompts/resume_generation.v1.tmpl
with the following content exactly as provided — do not modify or improve them.

/prompts/jd_extraction.v1.tmpl:
---
You are a technical recruiting analyst. Your job is to extract structured
signals from job descriptions to help engineers tailor their resumes.

Analyze the job description and extract the following:
- priority_skills: the technical skills most emphasized, in order of apparent
  importance. Include languages, frameworks, platforms, and methodologies.
  Normalize to canonical names (e.g. "Golang" -> "Go").
- seniority: the industry-standard level implied by the role. One of:
  junior, mid, senior, staff, principal, lead, manager, director, vp.
- domain_vocabulary: terms specific to this company or domain that a strong
  resume should mirror. Include product names, architectural terms, and
  methodology names the JD uses distinctively.

Return only valid JSON matching this structure, with no preamble or explanation:
{
  "priority_skills": ["string"],
  "seniority": "string",
  "domain_vocabulary": ["string"]
}

<job_description>
{{.JobDescription}}
</job_description>
---

/prompts/resume_generation.v1.tmpl:
---
You are an expert technical resume writer specializing in senior and staff-level
software engineering roles. You write with precision, confidence, and specificity.
You never fabricate experience or embellish outcomes. You compress and reframe
real experience to match what a role is looking for.

Your output must be valid JSON matching the resume schema provided. Return only
the JSON document with no preamble, explanation, or markdown formatting.

RULES:
- Every bullet must begin with a strong past-tense action verb
- Quantify outcomes wherever the source material supports it
- Mirror the domain vocabulary from jd_signals where it is genuine and accurate
- Do not invent metrics, technologies, or outcomes not present in the source material
- Prefer specificity over generality: "reduced p99 latency from 800ms to 120ms"
  beats "improved system performance"
- For each employer, include 3-5 bullets per position unless the tenure was under
  6 months, in which case 2-3 is appropriate
- Include projects only if their tags overlap meaningfully with priority_skills,
  unless force_include is true
- Exclude projects where force_exclude is true regardless of relevance
- The summary should be 2-3 sentences, written in third person, targeting the
  specific role and company

Generate a tailored resume for the following application.

<target_role>
Company: {{.CompanyName}}
Role: {{.RoleTitle}}
</target_role>

<jd_signals>
{{.JDSignals}}
</jd_signals>

<identity>
{{.Identity}}
</identity>

<contributions>
{{.Contributions}}
</contributions>

<projects>
{{.Projects}}
</projects>

<education>
{{.Education}}
</education>

<credentials>
{{.Credentials}}
</credentials>

<resume_schema>
{{.ResumeSchema}}
</resume_schema>

<prior_feedback>
{{.PriorFeedback}}
</prior_feedback>
---

### .env.example
DATABASE_URL=postgres://localhost:5432/role_model?sslmode=disable
ANTHROPIC_API_KEY=your-key-here
PORT=8080
ENVIRONMENT=development

### .gitignore
Standard Go gitignore. Also ignore:
.env
*.local

### go.mod
Module name: github.com/shurikai/role-model

### CLAUDE.md
Populate with the content provided below exactly as written.

## Constraints
- No business logic, no database queries, no LLM calls
- Stubs compile cleanly with no errors
- No ORM
- No database/sql
- No gin, echo, or heavy frameworks
- No hardcoded config values
- Errors returned as structured JSON: {"error": "message", "code": "slug"}
- All handlers accept context
- Business logic does not live in handlers

## CLAUDE.md content

# Role Model

**Repository:** https://github.com/shurikai/role-model

A self-hostable, single-user service for AI-powered resume generation.
Stores detailed career history as structured data and generates tailored,
versioned resumes per job application using an LLM.

Designed with a clear path to multi-tenant deployment without requiring
a schema redesign.

## Status
Early development. Schema and JSON contract are defined. Service not yet built.

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
- /schema/resume.v1.json     — intermediate resume JSON schema (source of truth)
- /migrations/               — database migrations (source of truth for schema)
- /prompts/                  — LLM prompt templates
- CLAUDE.md                  — this file, read at the start of every CC session

## Data Model Decisions
- UUIDs for all primary keys, client-generated
- created_at / updated_at on all tables
- Soft deletes on contributions (is_active) and anywhere historical data has value
- user_id on all tenant-scoped tables
- JSONB for flexible blobs: jd_signals, generation_params, structured_output,
  edited_deltas
- Tags are user-defined with user-defined categories, normalized via aliases
- Positions carry both verbatim company title and industry-normalized level/role
  with a level_rationale field
- Contributions are richer than resume bullets: full_description, outcomes, and
  scale_context are separate fields
- Bullet traceability: each generated bullet in the JSON carries contribution_ids
  linking back to source contributions
- Feedback is scoped per resume version, not per contribution globally

## API Design
- REST
- JSON request/response
- Structured error responses: {"error": "message", "code": "slug"}
- Environment-based config, no hardcoded values
- Auth is stubbed for single-user now, designed for JWT-based auth later

## Conventions
- No ORM — use sqlc generated code against pgx native interface
- No database/sql — pgx native only
- No framework beyond chi — stdlib patterns otherwise
- All handlers receive a context, all DB calls respect context cancellation
- Config via environment variables, loaded at startup into a typed Config struct
- Business logic does not live in handlers

## Do Not
- Use an ORM
- Use database/sql directly
- Use gin, echo, or any heavy framework
- Hardcode any user identity, file paths, or config values
- Add dependencies without a clear justification
- Store rendered document files in the database
- Put business logic in HTTP handlers
- Modify prompt files without bumping the version number
- Modify /schema/resume.v1.json without updating the version field inside it

## Open Questions
- Blob storage interface for rendered documents (local disk now, S3 later)
- Auth implementation (stub for single-user, JWT for multi-user)
- Renderer service: Go-native vs Python/python-docx (deferred)
- Evaluation strategy for prompt quality across versions (deferred)
