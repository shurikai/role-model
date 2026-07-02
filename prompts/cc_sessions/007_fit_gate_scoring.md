# CC Session: Fit Gate and Scoring Pipeline

## Context

Role Model is a self-hostable Go REST API for AI-powered resume generation. The
two-stage LLM pipeline (Stage 1: JD signal extraction into `applications.jd_signals`,
Stage 2: resume synthesis) is already built and working.

This session implements the fit gate: a deterministic scoring pass that runs after
Stage 1 and before Stage 2 to evaluate whether a JD is worth generating a resume
for. The gate produces a persisted `fit_reports` row with two numeric scores and an
LLM-written prose narrative.

Stack: Go, chi router, pgx/v5 native (no ORM), sqlc, golang-migrate, Anthropic Go
SDK. Prompt files embedded via `go:embed`, versioned in
`internal/generation/prompts/`. Auth is JWT-based with `RequireAuth` middleware.
All tenant-scoped tables carry `user_id`. Client-generated UUIDs throughout.
Structured errors as `{"error": "...", "code": "..."}`. TEXT with named CHECK
constraints throughout — no native PostgreSQL ENUM types.

---

## Target: New Migration

Create migration `migrations/<next_number>_fit_reports.up.sql` and corresponding
`.down.sql`.

```sql
CREATE TABLE fit_reports (
    id                  UUID          PRIMARY KEY,
    user_id             UUID          NOT NULL REFERENCES users(id),
    application_id      UUID          REFERENCES applications(id),
    anti_pattern_passed BOOLEAN       NOT NULL,
    anti_pattern_hits   JSONB,
    technical_score     NUMERIC(5,2),
    technical_gaps      JSONB,
    preference_score    NUMERIC(5,2),
    preference_gaps     JSONB,
    narrative           TEXT,
    created_at          TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX ON fit_reports(user_id);
CREATE INDEX ON fit_reports(application_id);
```

**Constraints:**
- `application_id` is nullable — fit reports may be run before an application record
  exists. Do not enforce NOT NULL.
- No `updated_at` — fit reports are immutable. Re-running produces a new row.
- `anti_pattern_hits` is nullable JSONB; populated only when `anti_pattern_passed`
  is FALSE, containing the preference rows that triggered the gate failure.
- `technical_gaps` and `preference_gaps` are nullable JSONB arrays of gap
  descriptions, populated when scores are below perfect.
- `narrative` is nullable TEXT; populated only when `anti_pattern_passed` is TRUE
  (no narrative is generated for failed gates).

**.down.sql** drops the table and its indexes:
```sql
DROP TABLE IF EXISTS fit_reports;
```

---

## Target: sqlc Queries

Add query file `internal/db/queries/fit_reports.sql`. Required queries:

- `CreateFitReport` — insert all columns, return the full row
- `GetFitReport` — by id + user_id
- `ListFitReportsByApplication` — by application_id + user_id, ordered by
  created_at DESC
- `ListFitReports` — by user_id, ordered by created_at DESC

Run `sqlc generate` after adding queries. Verify no compile errors.

---

## Target: Scoring Logic

Create `internal/fitgate/scorer.go`. This package owns all deterministic scoring.
No LLM calls in this package — pure Go logic against DB data.

### Input types

```go
// FitInput is assembled by the handler before scoring begins.
type FitInput struct {
    JDSignals      JDSignals         // parsed from applications.jd_signals
    Skills         []db.Skill        // from skills table, user-scoped
    Preferences    []db.Preference   // from preferences table, user-scoped
}

// JDSignals mirrors the structured output of Stage 1 extraction.
// Use whatever fields Stage 1 currently extracts into applications.jd_signals —
// at minimum: required_tags, preferred_tags, domain, seniority_level, work_type.
// Parse from JSONB using encoding/json into this struct.
type JDSignals struct {
    RequiredTags   []string `json:"required_tags"`
    PreferredTags  []string `json:"preferred_tags"`
    Domain         string   `json:"domain"`
    SeniorityLevel string   `json:"seniority_level"`
    WorkType       string   `json:"work_type"`
    // Add any additional fields present in the current Stage 1 output schema
}
```

### Anti-pattern gate (`RunAntiPatternGate`)

```go
func RunAntiPatternGate(prefs []db.Preference, signals JDSignals) (passed bool, hits []db.Preference)
```

- Filter preferences to those with `sentiment = 'hard_exclude'`
- For each hard-exclude preference, check whether the JD signals match:
  - `preference_type = 'anti_pattern'`: match against domain, work_type, or
    free-text scan of JD signal fields
  - Return `passed = false` and the matching preference rows on first hit
- If no hard-exclude preferences match, return `passed = true, hits = nil`
- Matching is case-insensitive substring match against signal string fields.
  Do not use fuzzy matching or LLM inference — deterministic only.

### Technical fit score (`ScoreTechnicalFit`)

```go
func ScoreTechnicalFit(skills []db.Skill, signals JDSignals) (score float64, gaps []string)
```

- Build a set of tag names the user has from `skills` table (skill.tag_id →
  tag name; you will need to join or pre-resolve tag names before calling this
  function — pass resolved names in, not UUIDs)
- Required tags: each match contributes more weight than preferred tags
- Suggested weighting: required match = 2 points each, preferred match = 1 point
  each; score = points earned / points possible, expressed as 0.00–100.00
- Gaps: required tags with no match in skills; include in gaps list
- If JD has no required or preferred tags, return score = 100.00, gaps = nil

### Preference fit score (`ScorePreferenceFit`)

```go
func ScorePreferenceFit(prefs []db.Preference, signals JDSignals) (score float64, gaps []string)
```

- Exclude `hard_exclude` preferences (already handled by gate)
- For remaining preferences (positive/negative sentiment):
  - Match preference keywords against JD signal fields (domain, work_type,
    seniority_level, free-text fields)
  - Positive match: add `preference.weight` to earned points
  - Negative match: subtract `preference.weight` from earned points
  - No match on a positive preference: add to gaps list
- Score = (earned / max_possible) × 100, clamped to 0.00–100.00
- If no non-hard-exclude preferences exist, return score = 100.00, gaps = nil

**Do NOT implement:**
- Any LLM call inside the scorer package
- Any database access inside the scorer package — all data is passed in
- Any fuzzy or semantic matching — case-insensitive substring only
- Normalization or stemming of tag names

---

## Target: Prompt File

Create `internal/generation/prompts/fit_narrative.txt`:

```
You are a career fit analyst. You will be given a structured fit report for a job
application and asked to write a concise prose narrative summarizing the findings.

The narrative should:
- Open with a direct assessment: strong fit, moderate fit, or weak fit
- Summarize the technical alignment in one or two sentences, calling out key matched
  skills and any significant gaps
- Summarize the preference alignment in one or two sentences, noting what about the
  role matches or conflicts with stated preferences
- Close with a concrete recommendation: apply, apply with caveats, or pass

Rules:
- Be direct and specific. Reference actual skills and gaps from the input.
- Do not invent information not present in the input.
- Write in second person ("Your technical profile...", "This role aligns with...")
- Target length: 4–6 sentences total. Do not write more than a short paragraph.
- Return plain text only. No JSON, no markdown, no headers.

Input will be provided as a JSON object with these fields:
- anti_pattern_passed: boolean
- technical_score: number 0-100
- technical_gaps: array of tag name strings
- preference_score: number 0-100
- preference_gaps: array of preference description strings
- jd_summary: brief description of the role (company, title, domain)
```

---

## Target: Fit Service

Create `internal/fitgate/service.go`. This service orchestrates the full fit
evaluation lifecycle.

### `RunFitEvaluation(ctx, userID, applicationID uuid.UUID) (*db.FitReport, error)`

1. Load the application from DB; verify it belongs to the user
2. If `jd_signals` is null on the application, return an error — Stage 1 must run
   first. Do not trigger Stage 1 from within this service.
3. Parse `jd_signals` JSONB into `JDSignals` struct
4. Load all skills for the user from DB; resolve tag names via join or pre-fetch
5. Load all preferences for the user from DB
6. Run `RunAntiPatternGate` — if it fails:
   - Persist a `fit_report` row with `anti_pattern_passed = false`,
     `anti_pattern_hits` populated, all scores null, narrative null
   - Return the persisted row; do not proceed to scoring
7. Run `ScoreTechnicalFit` and `ScorePreferenceFit` concurrently using goroutines
   and a WaitGroup or errgroup
8. Load and embed `fit_narrative.txt` prompt
9. Call Anthropic API with the prompt and a JSON summary of scores and gaps
10. Parse the plain text narrative from the response
11. Persist the complete `fit_report` row
12. Return the persisted row

**Do NOT implement:**
- Any retry logic beyond what the existing pipeline uses
- Any modification to the application row — fit reports are separate records
- Any automatic triggering of Stage 2 based on fit scores — that decision
  belongs to the user

---

## Target: HTTP Handler

Register routes under `/applications/{applicationID}/fit` using the existing chi
router and `RequireAuth` middleware pattern.

**`POST /applications/{applicationID}/fit`**
- Extract `userID` from JWT context
- Call `fitgate.RunFitEvaluation(ctx, userID, applicationID)`
- Return the full fit report as JSON
- If `jd_signals` is null on the application, return 422 with:
  `{"error": "Stage 1 extraction required before fit evaluation", "code": "signals_missing"}`
- On evaluation error, return 500 with descriptive error

**`GET /applications/{applicationID}/fit`**
- Return all fit reports for the application, user-scoped, ordered by created_at DESC
- Return empty array (not 404) if no reports exist yet

**`GET /applications/{applicationID}/fit/{reportID}`**
- Return single fit report, user-scoped
- 404 if not found or belongs to different user

---

## Verification Steps

Run these after implementation, before considering the session complete:

```bash
# 1. Migration round-trip
migrate -path migrations -database "$DATABASE_URL" up
migrate -path migrations -database "$DATABASE_URL" down 1
migrate -path migrations -database "$DATABASE_URL" up

# 2. Schema introspection
psql $DATABASE_URL -c "\d fit_reports"
# Confirm: application_id is nullable, no updated_at column

# 3. sqlc compile
sqlc generate

# 4. Go build
go build ./...

# 5. Smoke test (replace token and IDs)
TOKEN="<your_jwt>"

# Create an application and run Stage 1 first
APP=$(curl -s -X POST http://localhost:8080/applications \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "company_name": "Acme Observability",
    "role_title": "Senior Backend Engineer",
    "jd_text": "We are looking for a senior backend engineer with strong Go and Kubernetes experience. PostgreSQL and Kafka a plus. We value async communication and small autonomous teams."
  }')
echo $APP
APP_ID=$(echo $APP | jq -r '.id')

# Run Stage 1 extraction (existing endpoint)
curl -s -X POST http://localhost:8080/applications/$APP_ID/extract \
  -H "Authorization: Bearer $TOKEN" | jq .

# Run fit evaluation
FIT=$(curl -s -X POST http://localhost:8080/applications/$APP_ID/fit \
  -H "Authorization: Bearer $TOKEN")
echo $FIT | jq .

# Verify report persisted
curl -s http://localhost:8080/applications/$APP_ID/fit \
  -H "Authorization: Bearer $TOKEN" | jq .

# Verify 422 on application with no signals
APP2=$(curl -s -X POST http://localhost:8080/applications \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"company_name": "No Signals Co", "role_title": "Engineer", "jd_text": "TBD"}')
APP2_ID=$(echo $APP2 | jq -r '.id')

curl -s -X POST http://localhost:8080/applications/$APP2_ID/fit \
  -H "Authorization: Bearer $TOKEN" | jq .
# Should return 422 signals_missing
```

All steps should pass. The fit report response should contain scores, gaps, and a
narrative paragraph. The 422 case should return the correct error code.

