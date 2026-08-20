# CC Session: Replace Blended Preference Score with a Hit List

## Context

`ScorePreferenceFit` currently collapses preference matching into a single
`0-100` float using three stacked mechanisms: a normalized weighted average,
a penalty subtraction for hard-gate hits, and a ceiling cap (`hardGateCeiling
= 25.0`) layered on top. This is the origin of most of the scoring bugs
fixed in prior sessions (unmatched negatives silently paying out a bonus,
hard-gate hits reading as a minor dip before the ceiling existed, the
`prefFieldsFor` routing collision). The arithmetic itself is the problem,
independent of any individual bug.

Design decision (confirmed in conversation, not open for reinterpretation
mid-session): **Axis 2 (preferences) should never compute a single number.**
It should return a plain diagnostic — which preferences matched, which
didn't, which conflicted, which hard-gate rows fired — and nothing else.
The model's job stops at producing that list; anything resembling an
at-a-glance score is a future frontend-only concern, out of scope here.

Axis 1 (`ScoreTechnicalFit`, `technical_score`) is unaffected and stays
exactly as-is. This session is scoped to `ScorePreferenceFit` and everything
downstream of its return value only.

Stack: Go (`internal/fitgate/scorer.go`, `internal/fitgate/service.go`),
Postgres migration, sqlc regen, one prompt template
(`internal/generation/prompts/fit_narrative.txt`), minimal frontend
(`frontend/src/routes/ApplicationDetail.tsx`,
`frontend/src/lib/types.ts`).

---

## Session Start (always)

```bash
git pull
gh issue list --state open
```

---

## Task 1: `ScorePreferenceFit` returns a hit list, not a score

In `internal/fitgate/scorer.go`:

- Delete `hardGateCeiling` and all of its usage.
- Delete the `earned`/`possible`/`base`/`penalty` arithmetic and the
  `math.Min` ceiling clamp. Delete the `math` import if nothing else in the
  file needs it.
- New signature returns four lists, no float:
  ```go
  func ScorePreferenceFit(prefs []db.Preference, signals JDSignals) (
      matches []db.Preference, gaps []db.Preference, conflicts []db.Preference, gateHits []db.Preference,
  )
  ```
  - `matches`: positive preferences where `matchesSignal` succeeded.
  - `gaps`: positive preferences where it didn't (unmet — JD simply doesn't
    mention it, not a conflict; keep the doc-comment distinction that
    already exists explaining why these are different).
  - `conflicts`: negative, non-hard-gate preferences where it succeeded.
  - `gateHits`: unchanged from current behavior — hard-gate preferences
    where it succeeded. Stays a structurally separate list, not folded into
    `conflicts` with a severity flag. This is deliberate: a hard-gate hit
    (e.g. clearance/defense-coded work) and an ordinary matched negative are
    different kinds of finding, not different weights of the same finding.
  - An unmatched negative preference still isn't reported anywhere (no
    change to that existing behavior — "avoided a stated dislike" isn't
    notable).
- `matches`, `gaps`, and `conflicts` were previously `[]string` (labels
  only). Change them to `[]db.Preference` so the full row (including
  `preference_type`) is available to the narrative and frontend, matching
  how `gateHits` already works. Confirm this doesn't break anything that
  currently expects bare label strings — grep before assuming.
- Update the function's doc comment to state plainly that this returns a
  diagnostic, not a score, and why (reference the arithmetic-bug history
  above briefly, one or two sentences, not a restatement of every prior
  session).

## Task 2: Update `fit_reports` schema

New migration `015_drop_preference_score.up.sql` /
`015_drop_preference_score.down.sql`:

```sql
-- up
ALTER TABLE fit_reports DROP COLUMN preference_score;
ALTER TABLE fit_reports ADD COLUMN preference_matches JSONB;

-- down
ALTER TABLE fit_reports ADD COLUMN preference_score NUMERIC(5,2);
ALTER TABLE fit_reports DROP COLUMN preference_matches;
```

Update `internal/db/queries/fit_reports.sql` (`CreateFitReport`) to drop
`preference_score` and add `preference_matches` in the column list and
values. Run sqlc regen (check `Makefile`/`CLAUDE.md` for the exact target,
likely `make sqlc` or `sqlc generate`) and confirm `internal/db/models.go`
and `internal/db/fit_reports.sql.go` picked up the change.

## Task 3: Wire the new shape through `service.go`

In `internal/fitgate/service.go`:

- `RunFitEvaluation`: drop `prefScore` from the `ScorePreferenceFit` call
  and everywhere it's used. Add `prefMatches`.
- `narrativeInput`: drop `PreferenceScore float64`, add
  `PreferenceMatches []db.Preference` (or whatever shape Task 1 settled on
  — keep consistent with how `TechnicalMatches` is already passed).
- `CreateFitReportParams`: drop `PreferenceScore`, add `PreferenceMatches`
  (marshaled the same way `technicalMatchesJSON` already is via
  `marshalRawNonEmpty`).
- `generateNarrative`'s signature and call site: drop `prefScore` param,
  add `prefMatches`.

## Task 4: Update the narrative prompt

`internal/generation/prompts/fit_narrative.txt` currently reads
`preference_score` as an input. Rewrite it to reason from
`preference_matches` / `preference_gaps` / `preference_conflicts` /
`anti_pattern_hits` directly — same pattern the prompt already uses for
`technical_matches`/`technical_gaps` as evidence rather than a number.
Make sure the prompt still clearly distinguishes a hard-gate hit
(`anti_pattern_hits`) from an ordinary conflict in the language it's
instructed to produce — that distinction is the whole point of keeping the
lists separate.

## Task 5: Minimal frontend update — visibility without a design pass

In `ApplicationDetail.tsx`, the current "Preference score: X/100" line
(around line 311-316) has no data source anymore. Replace it with a plain,
unstyled list render of the four categories — reuse the exact pattern
already used for `technical_gaps` (a `<ul>` under a bold label) for each of:
matches, gaps, conflicts, and hard-gate hits. Do **not** attempt scoring,
visual severity treatment beyond what already exists (the existing red
styling on `preference_conflicts` can stay for `conflicts`; hard-gate hits
can reuse the same red styling — do not invent a new color/severity
system), layout redesign, or collapsing/expanding UI. This is a stopgap so
the information is visible, not a design pass — a real design pass is
already planned separately. If a task in this list starts to feel like a
design decision rather than a mechanical swap, stop and leave it as plain
text rather than improvising.

Update `frontend/src/lib/types.ts`: drop `preference_score: number | null`,
add `preference_matches` matching whatever shape the API now returns.

## Task 6: Update tests

- `scorer_test.go`: rewrite every test asserting a specific
  `preference_score` value to instead assert list membership (e.g. "X is in
  conflicts", "Y is in matches", "gateHits contains Z"). Do not just delete
  the score assertions — replace them with equivalent coverage of the new
  return shape.
- `eval_test.go` and `testdata/cases/hard-gate-fires.json`: update expected
  shape to match. `hard-gate-fires.json` in particular is testing exactly
  the scenario this session changes the representation of (a hard-gate
  hit) — read it carefully before editing, this fixture should still prove
  gate hits are detected and kept separate from ordinary conflicts, just
  via the new fields.

---

## Do NOT

- Do not touch `ScoreTechnicalFit` or `technical_score` — Axis 1 is
  unaffected and out of scope.
- Do not add any scoring, weighting, or numeric summary of preference fit
  anywhere in the backend — not in `scorer.go`, not in the narrative
  prompt, not in the API response shape. If it computes a number from the
  preference lists, it doesn't belong in this session.
- Do not design a new frontend treatment for the four preference
  categories beyond reusing the existing gap/conflict list pattern
  already on the page. No new components, no icons, no severity color
  scale beyond red-for-conflicts-and-gate-hits.
- Do not retroactively reprocess or backfill existing `fit_reports` rows
  whose `preference_score` will be dropped — historical reports lose that
  column's data, which is expected and accepted.
- Do not change `is_hard_gate`, `weight`, or `sentiment` on the
  `preferences` table itself — this session only changes how the existing
  data is scored and returned, not the preference schema.

---

## Verification Steps

```bash
go build ./...
go test ./internal/fitgate/... -v
go test ./internal/generation/... -v
go test ./...
cd frontend && npm run build && npm run test
```

Session is complete when: `ScorePreferenceFit` has no numeric return value
anywhere in its signature or callers, `preference_score` no longer exists
in the schema or any Go/TS type, all four preference lists render on
`ApplicationDetail.tsx`, no existing test regresses, and both `go build
./...` and the frontend build are clean.
