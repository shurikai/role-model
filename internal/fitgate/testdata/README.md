# Fit gate eval fixtures

Fixtures for `eval_test.go`. `scorer_test.go` covers matching *mechanics* —
word boundaries, alternatives splitting, gate field routing. These cover
*outcomes*: given a known career profile and known JD signals, does the gate
reach the right conclusion?

Run the scorecard:

```sh
go test ./internal/fitgate -run TestFitEval -v
```

## Layers

**Layer A (here).** Fixed `jd_signals` in, scoring outcome out. Deterministic,
offline, no database and no API key, runs inside `make test`.

**Layer B (not built).** Raw JD markdown through Stage 1 extraction, checked
against the golden signals below. Needs an API key and a tolerance model for
LLM variance. Each case names its source JD in `jd_fixture` so Layer B has an
attachment point.

The signals in these cases are **hand-authored** against
`internal/generation/prompts/jd_extraction.tmpl` — its field contract, its
closed enums for `seniority` / `domain` / `work_type`, its canonicalization
rules, and its `" | "` alternatives delimiter. They are what a faithful
extraction *should* produce. Nothing yet verifies that it does; that is Layer B.

They are deliberately not reverse-engineered to match preference labels. Where a
JD says "24/7 on-call rotation" the fixture says that, not the persona's
"on-call heavy" — writing the fixture to match would hide the defect the
fixture exists to expose.

## Profile

`profile-sample.json` mirrors `database/sample/007_skills_preferences.sql` —
Morgan Reyes, the fictional persona. The sample dataset is used rather than the
private seed so these fixtures stay public and reproducible, and because its
skill depth is deliberately varied (8 expert, 22 proficient, 6 novice, 2
inactive, with years independent of proficiency).

The profile carries `proficiency` and `years_experience` even though
`ScoreTechnicalFit` cannot currently see either. `evalProfile.activeSkills`
models `ListActiveSkillTagNamesByUser` and drops them in one visible place, so
the loss is legible rather than baked into the fixture. When depth is plumbed
through, the fixture already holds the data.

## Case format

```json
{
  "name": "strong-match",
  "jd_fixture": "tests/fixtures/sample-strong-match-jd.md",
  "description": "...",
  "signals": { "...jd_signals..." },
  "expect": {
    "technical_score":      { "min": 80, "max": 100 },
    "technical_gaps":       { "must_include": [], "must_exclude": [], "empty": false },
    "preference_score":     { "min": 30 },
    "preference_gaps":      { "must_include": [] },
    "preference_conflicts": { "must_exclude": [] },
    "gate_passed":          true,
    "gate_hits":            { "must_include": [] }
  }
}
```

Every field of `expect` is optional; omitted fields are not asserted. Any object
may carry a `"comment"` explaining the assertion — it is ignored by the loader.

**Scores assert bands, not exact values.** #43 and #44 both require reweighting
the scorer, and exact-score assertions would fail on every legitimate tuning
change and be deleted within a week. A band says what a score has to *mean*
without freezing how it is computed.

**Lists assert membership.** `must_include` / `must_exclude` match an entry
whole or as one alternative inside a grouped entry, so an assertion naming
`TimescaleDB` sees a gap recorded as `"TimescaleDB | ClickHouse"`.

## Known gaps

A case with `"status": "known_gap"` asserts correct behavior the scorer does not
yet produce. It must also carry `issue` and `reason`. Known gaps are reported in
the scorecard but do not fail the build — they hold a defect's definition of
done rather than pretending it isn't there.

**A known gap that starts passing DOES fail the build.** That means someone
fixed the defect: remove the marker and close out the issue.

Current known gaps, all confirmed by running the harness rather than by reading
the code:

| Case | Issue | What it holds |
|---|---|---|
| `known-gap-adtech-exclude-cannot-fire` | #47 | The `adtech` hard exclude does not fire on the adtech JD. `domain` is a closed enum with no adtech value, and the industry survives only in `screening_summary.industry`, which the gate never reads. |
| `known-gap-domain-enum-loses-industry` | #45 | `logistics` (weight 9) and `supply chain` (weight 8) are reported as preference gaps on a freight logistics JD, for the same enum reason. |
| `known-gap-role-shape-conflicts` | #45 | `frontend`, `on-call heavy`, and `mandatory overtime` do not surface as conflicts on a frontend-majority, on-call-heavy JD. Worse than silence: an unmatched negative preference *earns* its weight, so the JD is scored as having avoided what it advertises. |
| `known-gap-depth-blind-scoring` | #44 | Full coverage at novice depth scores 100.0, identical to full coverage at expert depth. |

## Adding a case

1. Author the signals from a real JD where possible, and name it in
   `jd_fixture`. Use `"synthetic"` only when isolating one variable —
   `known-gap-depth-blind-scoring` does, to hold everything but depth constant.
2. Assert what the outcome has to *mean*, not what the current code happens to
   emit. If the assertion fails, that is a finding, not a fixture bug — mark it
   `known_gap` with an issue and a reason rather than loosening it to green.
3. Keep preference gaps and conflicts asserted separately. Collapsing them
   produces false conflict language, and that separation is a standing
   constraint of the system, not an implementation detail.
