# CC Session: Competency Level in Technical Fit Scoring

## Context

`skills.proficiency` (`novice`/`proficient`/`expert`) and
`skills.years_experience` exist in the schema but `ScoreTechnicalFit`
discards both before scoring ever sees them (see its doc comment in
`internal/fitgate/scorer.go`). Every requirement is currently binary: the
user either holds the skill at any level, or it's a gap. There is no way to
represent "you have this skill, but at a lower level than the JD is asking
for."

This session adds a third verdict, `partial`, driven by comparing an
inferred JD-side level against the ground truth's recorded proficiency.

Design decisions confirmed in conversation (not open for reinterpretation
mid-session):

1. **Level is a new, separate extraction field — `skill_levels` — not
   encoded into `required_skills`/`preferred_skills` strings.** Those stay
   `[]string`, untouched, per the existing constraint from session 028
   (resume generation and the frontend consume that shape directly).
   `skill_levels` is populated **only** when the JD's language for a
   specific skill explicitly signals depth (an adjective like "expert" /
   "familiar with", or a stated years figure) — not forced onto every
   skill. Most requirements will have no entry here, and that's correct,
   not a gap in extraction.
2. **The JD-side level uses the exact same three-value scale as
   `skills.proficiency`** — `novice` / `proficient` / `expert` — so
   comparison is a plain ordinal check with no separate translation layer.
   Extraction makes the judgment call at extraction time (this is not a
   bigger ask than `seniority` or `domain`, which are already inferred
   judgments).
3. **A partial match earns half credit.** Required: 1 point instead of 2.
   Preferred: 0.5 instead of 1. This is the plainest way to make a partial
   match "move the needle" without inventing a new weighting scheme
   alongside the existing point totals.

Stack: Go (`internal/fitgate/scorer.go`, `internal/generation/extractor.go`),
prompt template (`internal/generation/prompts/jd_extraction.tmpl`), no DB
migration needed (`skills.proficiency` already exists and is already
queried by `ListActiveSkillMatchTermsByUser` — confirm this in Task 1
before assuming a query change is needed).

---

## Session Start (always)

```bash
git pull
gh issue list --state open
```

Read `internal/fitgate/scorer.go`'s `satisfies` and `ScoreTechnicalFit` in
full before starting Task 2 — the level check has to slot into the existing
three-tier match-strength logic (direct/alias/category) without disturbing
it. Level and match-kind are orthogonal (you can have a direct-name match
that's still below the required level), so this is an additional axis, not
a replacement for match-kind.

---

## Task 1: Confirm proficiency is already available where scoring needs it

`internal/db/queries/skills.sql`'s `ListActiveSkillMatchTermsByUser` (the
query `service.go` uses to build `[]SkillTerm` for the scorer) currently
selects `t.name, t.aliases, t.category, c.aliases AS category_aliases` —
**not** `s.proficiency`. Add it:

```sql
SELECT t.name, t.aliases, t.category, c.aliases AS category_aliases, s.proficiency
FROM skills s
...
```

Regenerate sqlc. Add `Proficiency string` to `fitgate.SkillTerm` in
`scorer.go`, and update the one call site in `service.go` that builds
`[]SkillTerm` from the query rows.

Do not touch `years_experience` — it stays dropped. The design decision was
level (proficiency) only; years was explicitly rejected as the comparison
basis back when this was first discussed (years-in-a-JD reframed as a
level *signal* at extraction time, not a number compared directly).

## Task 2: Extraction — add `skill_levels`

In `internal/generation/extractor.go`, add to `JDSignals`:

```go
// SkillLevels holds an inferred proficiency requirement for individual
// entries in RequiredSkills/PreferredSkills, populated only when the JD's
// language for that specific skill explicitly signals depth (an adjective
// like "expert"/"familiar with", or a stated years figure). Most
// requirements will have no entry here — that is the expected, correct
// case, not a gap in extraction. Uses the same three-value scale as
// skills.proficiency (novice/proficient/expert) so comparison in
// ScoreTechnicalFit needs no translation layer.
SkillLevels []SkillLevel `json:"skill_levels"`
```
```go
type SkillLevel struct {
    Requirement string `json:"requirement"` // matches an entry (or OR-group) in RequiredSkills/PreferredSkills verbatim
    Level       string `json:"level"`        // "novice" | "proficient" | "expert"
    Signal      string `json:"signal"`       // the JD language that produced this inference, for auditability
}
```

`Requirement` matches the *original* `required_skills`/`preferred_skills`
entry string verbatim, including any `" | "` OR-group — this is a lookup
key against those arrays, not a new independent list. If the JD signals
depth on only one alternative within an OR-group ("expert Kafka, or
familiarity with Kinesis"), that's an edge case worth a rule in the prompt
(Task 3) rather than a silent guess in Go — decide there whether the level
applies to the specific alternative or the group as a whole.

## Task 3: Update `jd_extraction.tmpl`

Add a `skill_levels` section following the same documentation pattern as
`core_competencies` (rationale paragraph, then explicit rules, then a
worked example). Cover:

- Only emit an entry when the JD gives an explicit depth signal for that
  specific skill — an adjective ("expert", "deep understanding of",
  "familiar with", "exposure to") or a stated years figure. A bare skill
  name with no such language gets no entry.
- Map common JD language to the three-value scale explicitly in the prompt
  (give the model the rubric, don't make it invent one): e.g. "expert",
  "deep understanding of internals", "5+ years" → `expert`; ordinary
  unqualified requirement language with a moderate years figure ("2-3
  years", "solid experience with") → `proficient`; "exposure to",
  "familiarity with", "basic knowledge of" → `novice`.
- `requirement` must match the corresponding `required_skills` /
  `preferred_skills` entry string exactly, including `" | "` groups where
  applicable — state this explicitly, since a mismatched string silently
  breaks the lookup in Task 4 with no error.
- `signal` should quote or closely paraphrase the specific JD language that
  produced the inference, for auditability in the report.
- Address the OR-group-with-one-leveled-alternative edge case from Task 2
  explicitly, with a worked example, rather than leaving it ambiguous.

Add `skill_levels` to the example output shape near the bottom of the
template.

## Task 4: `ScoreTechnicalFit` — add the `partial` verdict

In `internal/fitgate/scorer.go`:

- Add `MatchPartial` alongside the existing `MatchDirect`/`MatchAlias`/
  `MatchCategory` — but note this is a different kind of distinction:
  match-kind (direct/alias/category) describes *how* evidence was found;
  partial describes *whether the level found is sufficient*. A match can be
  direct-but-partial. Don't collapse these into one enum; `SkillMatch` needs
  both the existing `Kind` and a new field for the level outcome — propose
  `LevelMet bool` (or equivalent) rather than overloading `Kind`, and note
  the reasoning either way in the doc comment so the next person doesn't
  "simplify" it back into one field.
- After `satisfies` finds evidence for a requirement, look up whether that
  requirement has a `skill_levels` entry. If not, behavior is **unchanged**
  — full credit, exactly as today. If it does:
  - Take the highest `proficiency` among the matched evidence skills
    (ordinal: expert > proficient > novice).
  - If evidence proficiency ≥ required level: full credit, as today.
  - If evidence proficiency < required level: this requirement is
    `partial`. Award half credit (1 point for required instead of 2, 0.5
    for preferred instead of 1) and record it in a new `Partial []SkillMatch`
    slice on `TechnicalFit` (parallel to `Matches`/`Gaps`, not merged into
    either — a partial is neither a clean match nor a clean gap and
    shouldn't be filed as one for the narrative to later have to
    re-distinguish).
  - `pointsPossible` is unaffected by this change — it already counts each
    entry once at full required/preferred weight; only `pointsEarned` and
    which bucket an entry lands in changes.
- A requirement with **no evidence at all** stays a plain gap regardless of
  whether it has a `skill_levels` entry — level only matters once presence
  is established. Don't let an unmatched requirement's level data leak into
  gap reporting (e.g. "gap: expert Kafka required" vs. plain "gap: Kafka" —
  decide this explicitly rather than by accident; recommend keeping gap
  reporting exactly as today, unchanged, since the level distinction only
  has meaning once something was actually found).

## Task 5: Downstream — narrative, `fit_reports`, frontend

- `fit_reports.technical_matches` and the narrative prompt
  (`fit_narrative.txt`) both currently consume `technical.Matches`/
  `technical.Gaps`. Add `technical.Partial` alongside them the same way —
  new column `technical_partial JSONB` (migration `016_technical_partial.up/
  down.sql`), new field through `service.go` the same pattern as
  `technical_matches` was added in migration 013.
- Update `fit_narrative.txt` to describe partial matches distinctly from
  full matches and gaps in the language it's instructed to produce — a
  partial match is real evidence with a real caveat, not a strength and not
  a hole.
- Frontend: same minimal-visibility standard as the Step 1 session — render
  a third `<ul>` for partial matches next to the existing gaps/matches
  lists on `ApplicationDetail.tsx`, reusing existing styling patterns
  (matches' green/neutral styling plus the existing gap styling, no new
  color invented for "partial" beyond what's already on the page — if
  nothing existing fits, plain text with a label is fine). No design pass.

## Task 6: Tests

- `scorer_test.go`: add cases for the level comparison itself — evidence
  above required level (full credit, unchanged path), evidence below (half
  credit, lands in `Partial`), no `skill_levels` entry at all (existing
  behavior, full regression check that nothing changed for the common
  case), and an OR-group with a level on only one alternative (exercises
  the Task 3 rule).
- `eval_test.go` / `testdata/cases`: confirm nothing existing regresses —
  none of the current fixtures have `skill_levels` populated, so they
  should all still produce identical scores before/after this change. That
  invariant (no `skill_levels` data in, zero behavior change) is worth its
  own explicit assertion, not just "existing tests still pass."

---

## Do NOT

- Do not change `required_skills`/`preferred_skills` from `[]string`.
- Do not add `years_experience` back into scoring — level (proficiency)
  only, per the confirmed design.
- Do not force a `skill_levels` entry for every skill — absence is the
  common, correct case.
- Do not collapse match-kind (direct/alias/category) and level-met into a
  single enum.
- Do not touch `ScorePreferenceFit`, Axis 2, or anything from the prior
  session (`032_preference_fit_hitlist_not_score.md`) — this session is
  scoped to Axis 1 (technical) only.
- Do not invent a fourth verdict, a numeric partial-credit scale beyond the
  fixed half-credit rule, or per-skill importance weighting — all
  explicitly rejected in design discussion.
- Do not attempt a frontend design pass — reuse existing list/styling
  patterns only, same standard as the prior session.

---

## Verification Steps

```bash
go build ./...
go test ./internal/fitgate/... -v
go test ./internal/generation/... -v
go test ./...
cd frontend && npm run build && npm run test
```

Session is complete when: a JD with an explicit depth signal on a skill the
user holds at a lower recorded proficiency produces a `partial` verdict
with half credit and appears in its own list end-to-end (extraction →
scoring → fit_reports → narrative → frontend), a JD with no `skill_levels`
data produces byte-for-byte identical scores to pre-session behavior, no
existing test regresses, and both `go build ./...` and the frontend build
are clean.
