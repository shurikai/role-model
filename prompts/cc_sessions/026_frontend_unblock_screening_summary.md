# CC Session: Frontend — Unblock Generation, Surface Screening Summary

## Context

Session 025 (backend) made `RunAntiPatternGate` advisory instead of
blocking: `fit_reports` are now always fully populated (technical score,
preference score, narrative, `screening_summary`) regardless of gate
outcome, and `applications.jd_signals` now carries a `screening_summary`
object from Stage 1 extraction — before the fit gate even runs.

The frontend (`frontend/src/routes/ApplicationDetail.tsx`) has not been
touched and still assumes the old blocking contract:

- `canGenerate = !!latestFitReport?.anti_pattern_passed` — generation is
  still locked behind gate passage, even though the backend no longer
  treats that as a gate.
- The Fit Report section branches its entire render on
  `anti_pattern_passed`: when false, it shows only a red "Hard gate failed"
  box and nothing else — even though the backend now sends full scores and
  a narrative alongside that flag.
- Neither `JdSignals` nor `FitReport` types include `screening_summary` —
  it doesn't exist in the frontend at all yet.

This session updates types and the `ApplicationDetail` route to match the
new backend contract, and surfaces `screening_summary` where it's actually
useful: immediately after Stage 1 extraction, before the user even decides
whether to run the fit gate — this is the "let the user self-screen before
committing to Stage 2" flow the backend change was built for.

Stack: React + TypeScript + Vite, TanStack Query, Vitest.

---

## Session Start (always)

```bash
git pull
gh issue list --state open
```

Confirm session 025's backend changes are present (`screening_summary`
column on `fit_reports`, field on `JDSignals`) before starting — if not yet
merged/pulled, stop and flag rather than building against a contract that
isn't there yet.

---

## Task 1: Types

In `frontend/src/lib/types.ts`:

Add a `ScreeningSummary` interface matching the backend struct:

```ts
export interface ScreeningSummary {
  location: string;
  work_arrangement: string;
  travel: string;
  industry: string;
  clearance_citizenship: string;
  other_flags: string[];
}
```

Add `screening_summary: ScreeningSummary | null` to `JdSignals` (populated
once Stage 1 extraction has run) and to `FitReport` (the persisted copy at
fit-evaluation time). Both should be nullable/optional to handle rows
created before this migration.

## Task 2: Surface screening summary right after extraction

In the "Job Description Signals" section of `ApplicationDetail.tsx`
(currently renders seniority/domain/work_type/required_skills/etc.), add a
screening summary block rendered whenever
`application.jd_signals.screening_summary` is present. This should be
visually distinct from the skills-match signals below it — it's answering a
different question ("should I even consider this") not "do I match."
Suggested treatment: a bordered callout above or beside the existing signal
list, showing location, work arrangement, travel, industry, and clearance
as labeled fields, and `other_flags` as a bullet list. Don't invent a
sentiment/color-coding scheme (red/green) for `other_flags` — the backend
prompt (session 025) deliberately keeps this section descriptive, not
judgmental, and the UI shouldn't editorialize on top of it either. Plain
presentation; the user forms their own judgment.

## Task 3: Un-gate resume generation

Replace:

```ts
const canGenerate = !!latestFitReport?.anti_pattern_passed;
```

with a check based on whether a fit report exists at all:

```ts
const canGenerate = !!latestFitReport;
```

Update the disabled-button tooltip and helper copy ("Generation is
available once the fit gate has been run and passed") to reflect that
generation is now available once a fit evaluation has been run — full stop,
not conditional on passing anything. Something like "Run the fit
evaluation first to see scores before generating" is accurate; "and
passed" is not, anymore.

## Task 4: Un-branch the Fit Report render

Currently the entire scores/gaps/narrative block only renders in the
`anti_pattern_passed` branch. Change this so technical score, technical
gaps, preference score, preference gaps/conflicts, narrative, and
`screening_summary` (if you choose to also show it here — see note below)
always render whenever `latestFitReport` exists, since the backend always
populates them now.

Keep an anti-pattern indicator, but change its framing and remove the
gating implication:
- Rename "Hard gate failed" → something like "Anti-pattern flag" or
  "Preference match flagged" — it's informational now, not a verdict.
- Keep it visually distinct (still worth noticing) but drop language like
  "failed" that implies a blocking outcome.
- Remove the separate "Hard gate passed" green box in the passing case —
  with nothing gated on it anymore, a persistent "passed" indicator when
  there's usually nothing to say is just noise. Only show the anti-pattern
  section at all when `anti_pattern_hits` is non-empty.

Decide during implementation whether to duplicate the screening summary
display here (from `latestFitReport.screening_summary`) or rely solely on
the Task 2 placement (from `application.jd_signals.screening_summary`) —
they should be identical data captured at slightly different times
(extraction vs. fit-eval), so duplicating risks drift being confusing if a
JD is re-extracted after a fit report already exists. Recommend: Task 2's
placement is the primary read surface (available earliest, no fit-eval
required); skip duplicating in the Fit Report section unless it reads
oddly without it.

## Task 5: Tests

Update or add Vitest coverage for `ApplicationDetail` (check existing test
file location/patterns — likely alongside other route tests) covering:
- Generate button is enabled once a fit report exists, regardless of
  `anti_pattern_passed` value.
- Scores/narrative render when `anti_pattern_passed` is `false`, not just
  when `true`.
- Screening summary renders when present on `jd_signals`, and the section
  is simply absent (not an empty box) when `screening_summary` is null
  (handles pre-migration rows gracefully).

---

## Do NOT

- Do not touch backend code (`internal/fitgate`, migrations, prompts) —
  this session is frontend-only, consuming an already-shipped contract.
- Do not add color-coded sentiment/severity styling to `other_flags` —
  keep presentation neutral per Task 2.
- Do not remove the anti-pattern hits display entirely — it's still useful
  signal, just no longer gating.

---

## Verification Steps

```bash
cd frontend
npm run build        # confirm no type errors from the new fields
npm test             # Vitest suite, including new/updated ApplicationDetail coverage
npm run dev           # manual check: load an application with a fit report
                       # where anti_pattern_passed = false, confirm scores/
                       # narrative/screening summary all render, and the
                       # Generate button is enabled
```

Session is complete when: generation is available whenever a fit report
exists (not conditional on gate passage), the fit report section always
shows full data when present, screening summary is visible right after
extraction, and `npm run build` / `npm test` pass clean.
