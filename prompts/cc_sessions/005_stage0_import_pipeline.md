# CC Session: Stage 0 Import Pipeline

## Context

Role Model is a self-hostable Go REST API for AI-powered resume generation. Career
history is stored as structured `contributions` in PostgreSQL, with employers,
positions, and tags as supporting tables. The two-stage LLM pipeline (Stage 1: JD
signal extraction, Stage 2: resume synthesis) is already built and working.

This session implements Stage 0: an LLM-assisted data entry pipeline that extracts
structured career entries from pasted resume text, enriches them with flags and
suggestions, and holds them for human review before anything writes to `contributions`.

Stack: Go, chi router, pgx/v5 native (no ORM), sqlc, golang-migrate, Anthropic Go
SDK. Prompt files are embedded via `go:embed` and versioned in
`internal/generation/prompts/`. Auth is JWT-based with `RequireAuth` middleware.
All tenant-scoped tables carry `user_id`. Client-generated UUIDs throughout.
Structured errors as `{"error": "...", "code": "..."}`.

---

## Target: New Migration

Create migration file `migrations/<next_number>_stage0_import.up.sql` (and
corresponding `.down.sql`) adding two tables.

### `import_batches`

```sql
CREATE TABLE import_batches (
    id          UUID        PRIMARY KEY,
    user_id     UUID        NOT NULL REFERENCES users(id),
    raw_text    TEXT        NOT NULL,
    status      TEXT        NOT NULL DEFAULT 'pending'
                            CONSTRAINT import_batches_status_check
                            CHECK (status IN (
                                'pending',
                                'extracting',
                                'enriching',
                                'review',
                                'complete',
                                'failed'
                            )),
    error_text  TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### `contribution_drafts`

```sql
CREATE TABLE contribution_drafts (
    id               UUID        PRIMARY KEY,
    user_id          UUID        NOT NULL REFERENCES users(id),
    batch_id         UUID        NOT NULL REFERENCES import_batches(id),
    employer_name    TEXT        NOT NULL,
    position_title   TEXT        NOT NULL,
    summary          TEXT,
    full_description TEXT,
    outcomes         TEXT,
    scale_context    TEXT,
    flags            JSONB,
    status           TEXT        NOT NULL DEFAULT 'pending'
                                 CONSTRAINT contribution_drafts_status_check
                                 CHECK (status IN (
                                     'pending',
                                     'approved',
                                     'rejected'
                                 )),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

**Constraints:**
- All enum-like columns use TEXT with named CHECK constraints. No native PostgreSQL
  ENUM types anywhere in this schema.
- `error_text` is nullable; populated only on `failed` status.
- `flags` is nullable JSONB; populated by Stage 0b enrichment pass.
- No `position_id` FK on `contribution_drafts` — employer/position are free text for
  display context only. The real position linkage is supplied at approval time.

**Do NOT implement:**
- Any auto-matching of `employer_name`/`position_title` to existing employers or
  positions tables
- Any auto-creation of employers or positions from draft data
- Any trigger or function to auto-update `updated_at` — handle in application code

**.down.sql** must drop both tables in reverse dependency order:
`contribution_drafts` first, then `import_batches`.

---

## Target: sqlc Queries

Add query files for both tables following existing patterns in `internal/db/queries/`.
Use pgx/v5 named parameters. Required queries:

**import_batches:**
- `CreateImportBatch` — insert
- `GetImportBatch` — by id + user_id
- `UpdateImportBatchStatus` — update status + error_text + updated_at by id + user_id
- `ListImportBatches` — by user_id, ordered by created_at DESC

**contribution_drafts:**
- `CreateContributionDraft` — insert
- `GetContributionDraft` — by id + user_id
- `ListContributionDraftsByBatch` — by batch_id + user_id
- `UpdateContributionDraft` — update all editable fields (summary, full_description,
  outcomes, scale_context, flags, updated_at) by id + user_id
- `UpdateContributionDraftStatus` — update status + updated_at by id + user_id

Run `sqlc generate` after adding queries. Verify no compile errors before proceeding.

---

## Target: Prompt Files

Create two prompt files in `internal/generation/prompts/`:

### `stage0a_extraction.txt`

This prompt drives Stage 0a: extracting structured career entries from raw resume text.

```
You are a structured data extraction assistant. Your task is to extract career
history entries from the provided resume text and return them as a JSON array.

For each distinct position (employer + role combination), extract:
- employer_name: the name of the employer as it appears in the resume
- position_title: the job title as it appears in the resume
- summary: a single sentence summarizing the role and primary responsibility
- full_description: a paragraph describing the work in detail, preserving any
  specific technologies, outcomes, or scale context mentioned in the source text
- outcomes: measurable results or achievements, if present; null if none stated
- scale_context: team size, user counts, data volumes, or other scale signals,
  if present; null if none stated

Rules:
- Extract only what is explicitly stated. Do not infer, embellish, or invent
  specific numbers, technologies, or outcomes not present in the source text.
- If a field has no basis in the source text, set it to null.
- Preserve specific numbers and metrics exactly as written.
- Return ONLY a valid JSON array. No preamble, no explanation, no markdown fences.

Example output shape:
[
  {
    "employer_name": "Acme Corp",
    "position_title": "Senior Software Engineer",
    "summary": "...",
    "full_description": "...",
    "outcomes": "...",
    "scale_context": "..."
  }
]
```

### `stage0b_enrichment.txt`

This prompt drives Stage 0b: enriching a single extracted draft with flags,
suggestions, and gap identification.

```
You are a career history enrichment assistant. You will be given a single extracted
career entry and asked to review it for quality, completeness, and potential issues.

Return a JSON object with a single key "flags" containing an array of flag objects.
Each flag has:
- type: one of "inference", "gap", "suggestion", "warning"
- field: the field this flag applies to (employer_name, position_title, summary,
  full_description, outcomes, scale_context) or "general"
- message: a short, specific, actionable description of the issue or suggestion

Flag types:
- inference: something stated as fact that may have been inferred rather than
  explicitly sourced (e.g. a technology implied but not named)
- gap: a field that is null or thin where more detail would strengthen the entry
- suggestion: a concrete improvement that could make the entry more compelling
  without inventing new facts
- warning: something that appears inconsistent, implausible, or that the human
  reviewer should verify before approving

Rules:
- Do not suggest inventing specific numbers or outcomes not present in the source.
- Do not rewrite the entry. Only flag issues and suggest improvements.
- Return ONLY a valid JSON object with a "flags" array. No preamble, no markdown.
- If there are no flags, return: {"flags": []}

Input will be provided as a JSON object with the same fields as the extraction output.
```

---

## Target: Service Layer

Create `internal/stage0/service.go` (or equivalent location consistent with existing
service organization). This service owns the full Stage 0 lifecycle.

### Stage 0a — Extraction (`RunExtraction`)

1. Accept `batchID uuid.UUID` and `userID uuid.UUID`
2. Load the batch from DB; verify it belongs to the user and is in `pending` status
3. Update batch status to `extracting`
4. Load and embed `stage0a_extraction.txt` prompt
5. Call Anthropic API with the prompt + `batch.raw_text` as user content
6. Parse the JSON array response into draft structs
7. For each extracted entry, insert a `contribution_draft` row with status `pending`
8. Update batch status to `enriching`
9. Call `RunEnrichment` for each draft (see below)
10. Update batch status to `review` on success, `failed` (with error_text) on any error

### Stage 0b — Enrichment (`RunEnrichment`)

1. Accept a single `contributionDraft`
2. Load and embed `stage0b_enrichment.txt` prompt
3. Call Anthropic API with the prompt + draft fields as JSON user content
4. Parse the `flags` JSON array from the response
5. Update the draft's `flags` column with the parsed result

**Error handling:**
- If Stage 0a extraction fails (bad JSON, API error), set batch status to `failed`
  with descriptive `error_text`. Do not partially insert drafts.
- If Stage 0b enrichment fails for a single draft, log the error and continue —
  a missing flags column is acceptable; a failed batch is not.
- Use the existing Anthropic SDK client pattern from the Stage 1/2 pipeline.

**Do NOT implement:**
- Any automatic approval or write to `contributions`
- Any retry logic beyond what the existing pipeline uses
- Background goroutines or async processing — Stage 0 runs synchronously per request

---

## Target: HTTP Handlers

Register all routes under `/import` using the existing chi router and `RequireAuth`
middleware pattern. All handlers extract `userID` from JWT context.

### Batch endpoints

**`POST /import`**
- Body: `{"raw_text": "..."}`
- Generate a client UUID for the batch
- Insert the batch with status `pending`
- Call `stage0.RunExtraction(batchID, userID)` synchronously
- Return `{"id": "<batch_id>", "status": "review", "draft_count": N}`
- On extraction failure, return the batch with `status: "failed"` and `error_text`

**`GET /import/{batchID}`**
- Return batch row plus counts: total drafts, pending, approved, rejected

**`GET /import/{batchID}/drafts`**
- Return all drafts for the batch, user-scoped
- Verify batch belongs to user before returning drafts

### Draft endpoints

**`GET /import/drafts/{draftID}`**
- Return single draft, user-scoped

**`PUT /import/drafts/{draftID}`**
- Body: any subset of `{summary, full_description, outcomes, scale_context}`
- Partial update — only update fields present in the request body
- Update `updated_at`
- Return updated draft

**`POST /import/drafts/{draftID}/approve`**
- Body: `{"position_id": "<uuid>"}`
- Verify `position_id` exists and belongs to the authenticated user
- Insert a new row into `contributions` using draft field values
- Set `is_active = TRUE`
- Update draft status to `approved`
- Return `{"contribution_id": "<new_uuid>"}`

**`POST /import/drafts/{draftID}/reject`**
- No body required
- Update draft status to `rejected`
- Return `{"id": "<draft_id>", "status": "rejected"}`

**Do NOT implement:**
- Batch-level approve-all endpoint
- Any endpoint that bypasses the `position_id` requirement on approval
- Any modification to `contribution_tags` at approval time — tag linking is manual
  after import, same as existing contribution creation flow

---

## Verification Steps

Run these after implementation, before considering the session complete:

```bash
# 1. Migration round-trip
migrate -path migrations -database "$DATABASE_URL" up
migrate -path migrations -database "$DATABASE_URL" down 1
migrate -path migrations -database "$DATABASE_URL" up

# 2. Schema introspection
psql $DATABASE_URL -c "\d import_batches"
psql $DATABASE_URL -c "\d contribution_drafts"

# Confirm:
# - import_batches.status has named CHECK constraint (not a pg enum)
# - contribution_drafts.status has named CHECK constraint (not a pg enum)
# - contribution_drafts has no position_id column

# 3. sqlc compile
sqlc generate

# 4. Go build
go build ./...

# 5. Smoke test the full pipeline via curl (replace token and position_id)
TOKEN="<your_jwt>"
POSITION_ID="<a_real_position_uuid>"

# Create a batch and run extraction
BATCH=$(curl -s -X POST http://localhost:8080/import \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"raw_text": "Senior Software Engineer at Acme Corp (2020-2023). Built distributed event processing system in Go handling 50k events/sec. Reduced p99 latency by 40%. Team of 4."}')
echo $BATCH
BATCH_ID=$(echo $BATCH | jq -r '.id')

# List drafts
curl -s http://localhost:8080/import/$BATCH_ID/drafts \
  -H "Authorization: Bearer $TOKEN" | jq .

DRAFT_ID=$(curl -s http://localhost:8080/import/$BATCH_ID/drafts \
  -H "Authorization: Bearer $TOKEN" | jq -r '.[0].id')

# Edit a draft field
curl -s -X PUT http://localhost:8080/import/drafts/$DRAFT_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"summary": "Updated summary text."}' | jq .

# Approve — writes to contributions
curl -s -X POST http://localhost:8080/import/drafts/$DRAFT_ID/approve \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"position_id\": \"$POSITION_ID\"}" | jq .

# Confirm contribution exists
curl -s http://localhost:8080/contributions \
  -H "Authorization: Bearer $TOKEN" | jq .
```

All curl steps should return 200 with expected shapes. The final contributions
list should include the newly approved entry.
