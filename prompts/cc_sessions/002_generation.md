Context: Role Model, Go service. Read CLAUDE.md. Stage 1 (JD signal extraction)
is working. This task implements Stage 2: resume generation. The context
assembly layer (AssembleContext on generation.Service) and identity (GetUser,
users table identity columns) already exist.

Task: Implement resume generation as a method on generation.Service, plus its
HTTP endpoint. Read internal/generation/*.go and internal/api/handlers/*.go
first to match existing patterns exactly.

## The method
func (s *Service) Generate(ctx context.Context, applicationID, userID uuid.UUID) (*db.ResumeVersion, error)

Steps in order:
1. GetApplication(applicationID, userID). If jd_signals is nil, return a sentinel
   error indicating signals must be extracted first (the handler maps this to 409
   or 400 — see handler section).
2. AssembleContext(userID) for career data.
3. GetUser(userID) for identity.
4. Render the resume_generation.v1.tmpl prompt. The template's data struct needs
   fields for: CompanyName, RoleTitle, JDSignals, Identity, Experience (the
   assembled ResumeContext), ResumeSchema, and empty placeholders for Projects,
   Education, Credentials, PriorFeedback (pass empty/"[]" for now — assembly does
   not yet produce these). Serialize structs to indented JSON for insertion.
5. Embed schema/resume.v1.json via go:embed (mirror how prompts are embedded) and
   insert it into the ResumeSchema slot.
6. Call the Anthropic API via the existing Client (same model/pattern as
   ExtractSignals).
7. Strip markdown code fences from the response (reuse the existing stripCodeFence
   helper) and parse as JSON.
8. Validate the parsed JSON against resume.v1.json using
   santhosh-tekuri/jsonschema. On validation failure, return an error that
   includes the validation detail. Do NOT retry — fail and report.
9. Compute next version_number: a sqlc query returning
   COALESCE(MAX(version_number),0)+1 for the application.
10. Store a resume_version: application_id, user_id, version_number,
    structured_output (the validated JSON), generation_params (JSON with model
    name and prompt_version "v1"). Return the stored row.

## sqlc queries to add (internal/db/queries/resume_versions.sql)
- NextResumeVersionNumber: SELECT COALESCE(MAX(version_number),0)+1 for an
  application scoped by user_id.
- CreateResumeVersion: INSERT ... RETURNING *.
Run sqlc generate. Note: generation_params and structured_output already have
*json.RawMessage / json.RawMessage overrides configured.

## The handler
internal/api/handlers, new method on a generation handler (or extend the existing
one that has ExtractSignals):
- POST /api/v1/applications/{id}/generate
- Parse and validate the UUID (match existing handlers).
- Call Service.Generate with stubUserID.
- Map the "no signals" sentinel error to 409 Conflict with a clear code/message.
- Map Anthropic failures to 502 (match ExtractSignals).
- Map schema-validation failures to 502 with code "invalid_generation" and a
  message; log the validation detail server-side.
- On success, 201 Created with the resume_version.
Wire the route in router.go.

## Constraints
- Match existing patterns: handler structure, WriteError/WriteJSON helpers,
  error wrapping, stubUserID, ErrNoRows->404.
- Do not modify the prompt template, schema, or AssembleContext.
- Do not add retry logic, caching, or goroutines.
- Do not implement projects/education/credentials assembly — pass empty.
- structured_output is NOT NULL; generation_params is nullable.

## Tests
Integration test behind //go:build integration: against the seeded DB and a real
application that already has jd_signals, call Generate and assert a resume_version
is stored with version_number 1 and non-empty structured_output. Skip if
DATABASE_URL or ANTHROPIC_API_KEY is unset.
