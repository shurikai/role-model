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
`seniority` list (rendered from the user's `career_levels` rows), its
canonicalization rules, and its `" | "` alternatives delimiter. `domain` and
`work_type` were closed enums here too until migration 021 deleted them; the
posting's own words now live in `screening_summary.industry` and
`screening_summary.work_arrangement`. They are what a faithful
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

The profile carries `proficiency` and `years_experience`.
`evalProfile.activeSkills` models `ListActiveSkillMatchTermsByUser` exactly:
proficiency now reaches the scorer, and `years_experience` is still dropped
there, in the one place that models the production query, so the loss stays
legible rather than baked into the fixture.

Proficiency is only *consulted* where a posting states a depth for a specific
requirement (`signals.skill_levels`). Every fixture written before that field
existed states none, so proficiency is available and unused in all of them —
which is why they still score exactly what they scored before.

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
    "technical_partial":    { "must_include": [], "must_exclude": [] },
    "preference_matches":   { "must_include": [] },
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
without freezing how it is computed. Only `technical_score` is a score;
preference fit has none.

**Preference fit is four lists, not a number.** `preference_matches`,
`preference_gaps`, `preference_conflicts`, and `gate_hits` each assert
membership, and a case names the preference it expects rather than a threshold
standing in for one. Two cases used to carry a `preference_score` band —
`hard-gate-fires` asserted a ceiling and `regression-or-group-primary-language`
a floor set just above that ceiling — and both now say directly what the band
was a proxy for.

**A gate hit appears in `gate_hits` and nowhere else.** It is no longer copied
into `preference_conflicts`; the duplication existed so a single score could be
narrated, and the two lists are separate precisely because a disqualifier and an
ordinary matched negative are different kinds of finding.

**Lists assert membership.** `must_include` / `must_exclude` match an entry
whole or as one alternative inside a grouped entry, so an assertion naming
`TimescaleDB` sees a gap recorded as `"TimescaleDB | ClickHouse"`.

**Skill levels are opt-in, and the harness asserts it.** A case whose `signals`
carry no `skill_levels` must produce an empty `technical_partial`; `TestFitEval`
fails the case outright if it does not, independently of anything in `expect`.
That is what keeps the depth comparison additive — every fixture here predates
the feature, and all of them must keep scoring what they scored before it
existed. `partial-match-depth-below-stated-level` is the positive counterpart,
and the only case that states a depth at all.

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
| `known-gap-adtech-exclude-cannot-fire` | #48 | The `adtech` hard exclude does not fire on the adtech JD. The routing half closed with migration 021 — the `dealbreaker` branch reads `screening_summary.industry` now — so what is left is lexical: `adtech` is not a whole-word run inside "programmatic advertising technology, demand-side platform". |
| `known-gap-domain-enum-loses-industry` | #53 | `logistics` now matches through `screening_summary.industry`; `supply chain` (weight 8) is still reported as a gap on a freight logistics JD, because that phrase appears nowhere in the signals. Preference labels have no aliases column. |
| `known-gap-role-shape-conflicts` | #45 | `frontend`, `on-call heavy`, and `mandatory overtime` do not surface as conflicts on a frontend-majority, on-call-heavy JD. The bonus an unmatched negative used to earn is gone with the score, but the silence is not: the report still says nothing about what the posting advertises. |
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
