# CC Session: Screening Summary Replaces Blocking Anti-Pattern Gate

## Context

Role Model's fit pipeline currently uses `RunAntiPatternGate`
(`internal/fitgate/scorer.go`) to hard-block resume generation: if a JD's
extracted `domain`/`work_type`/`seniority`/`culture_signals` fields
substring-match a `hard_exclude` preference, `RunFitEvaluation`
(`internal/fitgate/service.go`) short-circuits and persists a bare
`fit_reports` row with `anti_pattern_passed = false` and no scores or
narrative — the JD never gets scored, and nothing is surfaced beyond which
preference matched.

This has two compounding problems, discovered via a real JD (Twilio
"Principal Software Engineer, Data Substrate") that should have hard-failed
but didn't, because nothing in `JDSignals` could represent "this team serves
internal engineers, not external customers":

1. **The schema problem**: `JDSignals`' fixed fields (`domain` as a business-
   vertical enum, `work_type` as remote/hybrid/onsite) can't represent most
   real-world exclude criteria — audience (internal vs. external), timezone
   overlap requirements, travel expectations, clearance requirements, and
   whatever comes next. Every new exclude category has needed a new field.
   That list is unbounded and will keep growing.
2. **The blocking problem**: even a correct match makes an irreversible
   decision — no resume, no score, no narrative — from a single
   substring-match check on thin, lossy extracted data. There's no path to
   review or override a wrong call.

**Decision: stop trying to solve this with more fields and more matching
logic.** Jason (the user) reads every JD before seriously considering it and
reliably pattern-matches undesirable roles (defense language, wrong
timezone, wrong industry, etc.) faster and more reliably than any keyword
scheme can. The fix is to have Stage 1 extraction produce a **screening
summary** — the plain-language facts a human scans for — and always surface
it, rather than trying to encode an ever-growing exclude taxonomy into
`JDSignals` fields and match logic.

Stack: Go, chi router, pgx/v5, sqlc, golang-migrate, Anthropic Go SDK,
prompts embedded via `go:embed` in `internal/generation/prompts/`.

---

## Session Start (always)

```bash
git pull
gh issue list --state open
```

---

## Task 1: Add `ScreeningSummary` to Stage 1 extraction

Extend `JDSignals` in `internal/generation/extractor.go`:

```go
// ScreeningSummary holds plain-language facts a human would scan a JD for
// before seriously considering it — criteria that don't relate to skills
// match but often decide whether a role is worth pursuing at all. This is
// deliberately descriptive, not classificatory: no fixed enums, because the
// set of things worth flagging is open-ended and person-specific.
type ScreeningSummary struct {
    Location            string   `json:"location"`             // e.g. "Providence, RI" or "Remote, US" or "not stated"
    WorkArrangement      string   `json:"work_arrangement"`     // e.g. "fully remote", "hybrid, 2 days/week Minneapolis", "onsite"
    Travel               string   `json:"travel"`               // e.g. "occasional customer site visits" or "not mentioned"
    Industry             string   `json:"industry"`             // plain-language description, e.g. "defense/autonomous systems", "themed entertainment", "developer telemetry SaaS" — not constrained to a fixed list
    ClearanceCitizenship string   `json:"clearance_citizenship"`// e.g. "U.S. citizenship + active clearance required" or "not mentioned"
    OtherFlags           []string `json:"other_flags"`          // free text: anything else identifying or discriminating that isn't a skills match — military-coded language, FedRAMP mentions, "serves internal engineering teams" framing, anonymous/no-named-employer posting, unusual comp structure (commission-heavy, equity-only), notable red flags, etc. Empty array if nothing notable.
}
```

Add `ScreeningSummary ScreeningSummary `json:"screening_summary"`` as a new
field on `JDSignals`.

Update `internal/generation/prompts/jd_extraction.tmpl` (this file is
unversioned per its own header comment — edit in place, do not create a
`.v3` file) to add `screening_summary` to the returned JSON shape. Include:

- Clear instructions per field matching the struct doc comments above.
- At least 2-3 worked examples of `other_flags` entries spanning different
  categories (industry framing, audience/customer type, compliance/clearance,
  posting-quality red flags) so the model understands the field's scope isn't
  limited to one dimension.
- Explicit instruction that `other_flags` should be inferred from framing and
  language even when not stated in so many words — e.g. "platform for our
  engineering teams" or "shared services" implies an internal-engineering
  audience even without the word "internal."
- Explicit instruction: this section describes the role, it does not judge
  it. No sentiment, no "this is a red flag because..." — plain facts only.
  Judgment stays with the human reading it.

Update the example output shape at the bottom of the template to include a
`screening_summary` object.

## Task 2: Stop blocking on the anti-pattern gate

In `internal/fitgate/service.go`, `RunFitEvaluation`:

- Remove the short-circuit branch that returns early on `!passed`.
- Always proceed to scoring (`ScoreTechnicalFit`, `ScorePreferenceFit`) and
  narrative generation regardless of `RunAntiPatternGate`'s result.
- Still call `RunAntiPatternGate` and still persist `anti_pattern_passed`
  and `anti_pattern_hits` on the report — this data is still useful as a
  flag, just no longer authoritative or blocking.
- Persist the new `screening_summary` (marshaled JSONB) on every report,
  regardless of gate outcome.

`RunAntiPatternGate` and `ScorePreferenceFit` themselves
(`internal/fitgate/scorer.go`) do not need logic changes — leave the
existing substring matching as-is. It's now a lightweight assist (still
occasionally useful when it happens to hit) rather than the primary
mechanism, so it's not worth further investment right now.

## Task 3: Migration

New migration `migrations/010_fit_report_screening_summary.up.sql` /
`.down.sql`:

```sql
ALTER TABLE fit_reports ADD COLUMN screening_summary JSONB;
```

`.down.sql`:
```sql
ALTER TABLE fit_reports DROP COLUMN screening_summary;
```

Regenerate sqlc after updating `internal/db/queries/fit_reports.sql` to
include the new column in `CreateFitReport` (insert) and all select queries.

## Task 4: Update the narrative prompt

`internal/generation/prompts/fit_narrative.txt` and the `narrativeInput`
struct in `internal/fitgate/service.go` (`generateNarrative`) should include
`screening_summary` in the input passed to the narrative LLM call, so the
prose narrative can reference it when relevant (e.g. "This is an onsite
Minneapolis role requiring clearance" folded naturally into the summary)
without necessarily restating every field — the raw `screening_summary`
object is the reliable source; narrative is a convenience layer, not the
place users should have to go to find these facts.

Since the gate no longer blocks, `generateNarrative` always runs — remove
any remaining code path or comment implying it's conditional on gate outcome.

## Task 5: Audit existing preferences for continued relevance

```bash
psql $DATABASE_URL -c "SELECT id, preference_type, label, sentiment FROM preferences WHERE user_id = '<seeded-user-id>' ORDER BY preference_type, sentiment;"
```

List every `hard_exclude` row. For each, note in the session summary whether
it's now redundant with `screening_summary` (i.e. purely a "human reads and
judges" criterion — audience, industry framing, clearance, timezone-type
concerns) versus still doing real work as a `required_skills`/technical
match (e.g. a primary-language exclude like Ruby/Node/Python-primary, which
*is* reliably machine-matchable against `required_skills` rather than the
fuzzy `domain`/`work_type`/`culture_signals` fields).

Do not delete or modify any preference rows. This is a read-only audit for
Jason's review — flag candidates, make no changes.

## Task 6: Re-run the Twilio JD as a smoke test

Use the Twilio "Principal Software Engineer, Data Substrate" JD text
(available in prior conversation history) to confirm:

- `screening_summary.other_flags` includes something capturing the internal-
  platform / internal-engineering-audience framing (wording doesn't need to
  match exactly — confirm it's *present and legible*, not that it hits an
  exact string).
- The fit report is fully populated (technical score, preference score,
  narrative, screening_summary) even though this would previously have
  hard-failed the gate.
- `anti_pattern_passed`/`anti_pattern_hits` still reflect whatever the
  existing preference-matching logic produces (informational only now).

This is the acceptance test: the JD produces a complete, readable report
that would let a human correctly self-screen it in a few seconds, rather
than either silently blocking or silently passing it through unflagged.

---

## Do NOT

- Do not remove `RunAntiPatternGate` or `ScorePreferenceFit` — they still
  run and still populate useful fields, just non-blocking now.
- Do not add fuzzy/semantic matching anywhere in `scorer.go`.
- Do not build frontend UI for displaying `screening_summary` in this
  session — that's part of the already-planned application-flow frontend
  work. This session's job is making sure the API response carries the data
  cleanly.
- Do not delete or edit preference rows during the Task 5 audit.

---

## Verification Steps

```bash
# 1. Migration round-trip
migrate -path migrations -database "$DATABASE_URL" up
migrate -path migrations -database "$DATABASE_URL" down 1
migrate -path migrations -database "$DATABASE_URL" up

# 2. sqlc + build
sqlc generate
go build ./...

# 3. Re-run extraction + fit against the Twilio JD, confirm full report
#    generated with screening_summary populated and no early-return on gate
curl -s -X POST http://localhost:8080/applications/$APP_ID/extract \
  -H "Authorization: Bearer $TOKEN" | jq .
curl -s -X POST http://localhost:8080/applications/$APP_ID/fit \
  -H "Authorization: Bearer $TOKEN" | jq .
# Expect: technical_score, preference_score, narrative, and screening_summary
# all populated, regardless of anti_pattern_passed value.

# 4. Regression: re-run a prior known-good application through fit,
#    confirm no behavior change in scoring for JDs that previously passed
#    the gate cleanly.

# 5. Existing test suites
go test ./...
```

Session is complete when the Twilio JD produces a full report with a
legible `screening_summary`, gate results are informational-only, the Task 5
audit is documented in the session summary, and `go build ./...` / `go test
./...` pass clean.
