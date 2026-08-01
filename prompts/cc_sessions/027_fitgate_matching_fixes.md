# CC Session: Fix Fragile Matching in fitgate — Word-Boundary Matching + Skills-Based Excludes

## Context

Session 025 made the anti-pattern gate advisory instead of blocking, which
incidentally defused (but did not fix) two real bugs in
`internal/fitgate/scorer.go`, both surfaced during that session's
acceptance testing:

**Bug A — substring false positives via `matchesSignal`.** The hard-exclude
preference labeled something like "IT consulting / staff augmentation
model" contains the substring "staff." `matchesSignal`'s bidirectional
`strings.Contains` check means this preference matches `seniority="staff"`
on every staff-level JD, regardless of whether the JD has anything to do
with consulting or staff augmentation. Confirmed directly:

```
seniority="staff"     passed=false  hits=1   (wrong — unrelated match)
seniority="senior"    passed=true   hits=0
seniority="principal" passed=true   hits=0
```

Under the old blocking behavior this silently killed every staff-level
application with no correct explanation. It's non-blocking now, but the
match itself is still wrong and will keep showing up as a misleading flag.

**Bug B — `required_skills`/`preferred_skills` are never checked by the
gate.** `signalFields()` only returns `Domain`, `WorkType`, `Seniority`,
and `CultureSignals`. It never reads `signals.RequiredSkills` or
`signals.PreferredSkills`. A prior session's Task 5 assumed primary-language
hard-excludes (Ruby, Node/TypeScript-primary, etc.) were "reliably
machine-matchable against required_skills" — that was wrong. Nothing reads
`required_skills` at all. An audit (from session 025's Task 5,
`notes/hard-exclude-preference-audit.md`) found 4 of 8 `hard_exclude`
preference rows are skills-shaped and have never been able to fire, ever.

This session fixes both. They're related — both are about making
`scorer.go`'s matching actually correct — but distinct enough to treat as
two clear sub-tasks.

Stack: Go, `internal/fitgate/scorer.go`. No DB or LLM calls in this
package — keep it that way.

---

## Session Start (always)

```bash
git pull
gh issue list --state open
```

Read `notes/hard-exclude-preference-audit.md` (from session 025) before
starting Task 2 — it already identifies which preference rows are
skills-shaped vs. classification-shaped; don't re-derive this from scratch.

---

## Task 1: Fix substring false positives (word-boundary matching)

Replace `matchesSignal`'s raw `strings.Contains` bidirectional check with
word-boundary-aware matching. The core problem: `"staff"` should not match
inside `"staff augmentation"` when the JD signal being compared is the
single token `"staff"` and the preference label is a multi-word phrase
about a business model, not a seniority level.

Approach: tokenize both the preference label and each signal field into
lowercase words (split on non-alphanumeric), and require that either://
(a) the full signal field (as a normalized phrase) appears as a contiguous
    word sequence within the label, or
(b) the full label (as a normalized phrase) appears as a contiguous word
    sequence within the signal field.

This preserves the existing bidirectional intent (short canonical signal
token vs. longer descriptive label, matching in either direction — see the
existing doc comment on `matchesSignal`) while preventing a short token
from matching merely because its characters appear inside an unrelated
longer word or phrase.

Concretely: `"staff"` (signal) must not match inside `"staff augmentation"`
(label) as a substring-of-characters, but `"staff augmentation"` (label,
normalized) also clearly isn't equal to or containing `"staff"` (signal) as
a *word sequence match in the other direction either — so under
word-boundary matching this pair correctly does not match. Meanwhile
`"defense"` (signal, e.g. domain="defense") should still match a label like
`"defense / aerospace"` since `"defense"` is a whole word within that
label.

Write this as a small helper, e.g.:

```go
// containsPhrase reports whether needle appears as a contiguous sequence
// of whole words within haystack (both normalized to lowercase word
// tokens). Unlike a raw substring check, this will not match "staff"
// inside "staff augmentation" as a false positive from character overlap
// alone when needle and haystack aren't meant to refer to the same thing —
// it requires an actual word-level match.
func containsPhrase(needle, haystack string) bool {
    // tokenize both, check for needle's word sequence as a contiguous
    // subsequence of haystack's words
}
```

Update `matchesSignal` to use this instead of `strings.Contains` in both
directions. Keep the function signature and doc comment intent (short
signal token vs. longer descriptive label, bidirectional) — only the
matching primitive changes.

**Add test coverage** (`scorer_test.go` — create if it doesn't exist,
following whatever test conventions are already used elsewhere in the repo,
e.g. `assemble_test.go`, `generate_test.go`) covering:
- The exact regression case: `seniority="staff"` must NOT match label
  `"IT consulting / staff augmentation model"` (use the actual label text
  from the audit notes if available, else this approximation).
- A true positive still works: `domain="defense"` matches label
  `"defense / aerospace"` (or whatever the actual stored label is).
- A few other word-boundary edge cases you think are worth covering (e.g.
  plural/singular mismatch behavior — decide and document whether that's
  in scope; if not, note it as a known limitation rather than silently
  under-testing it).

## Task 2: Wire `required_skills`/`preferred_skills` into the anti-pattern gate

Using the audit in `notes/hard-exclude-preference-audit.md`, identify which
`hard_exclude` preference rows are skills-shaped (primary-language/stack
excludes like Ruby, Node.js/TypeScript-primary, etc.) versus
classification-shaped (domain/culture/audience judgment calls, which per
the session 025 redesign are now intentionally left to the screening
summary and human review, not automated matching).

For the skills-shaped rows only: extend `RunAntiPatternGate` to also check
`signals.RequiredSkills` and `signals.PreferredSkills` using the same
word-boundary matching from Task 1 (via `containsPhrase` or equivalent),
not the raw `matchesAny` substring check used elsewhere in this file for
technical scoring (that one is a different, lower-stakes context — leave
`ScoreTechnicalFit`'s `matchesAny` alone).

Do NOT extend automatic gate matching to the classification-shaped rows
that session 025 deliberately moved to human review via
`screening_summary` — re-enabling automated matching for those would
partially undo that redesign. If you're unsure whether a given row is
skills-shaped or classification-shaped, treat it as classification-shaped
(leave it out of the skills check) and flag it in your session summary
rather than guessing.

**Add test coverage** confirming a JD with `required_skills` containing
e.g. `"Ruby on Rails"` correctly matches a Ruby-primary hard-exclude
preference, and that this didn't work before this change (i.e. the test
would have failed against the pre-fix `RunAntiPatternGate`).

---

## Do NOT

- Do not change `ScoreTechnicalFit`'s `matchesAny` — that's working
  correctly for its purpose (technical score matching, not gate matching)
  and isn't implicated in either bug.
- Do not extend automated matching to classification-shaped preferences
  (domain/audience/culture judgment calls) — those are intentionally
  human-reviewed now per session 025.
- Do not add fuzzy/semantic/LLM-based matching — word-boundary tokenized
  matching only, still fully deterministic.
- Do not modify preference rows in the database — if any labels would
  clearly work better reworded for matching (e.g. ambiguous short words),
  note it in the session summary as a suggestion rather than changing data.

---

## Verification Steps

```bash
go build ./...
go test ./internal/fitgate/... -v
go test ./...   # full regression
```

Session is complete when: the `seniority="staff"` false positive no longer
fires, a genuine defense/aerospace-style match still fires correctly, at
least one previously-dead skills-shaped hard-exclude now correctly fires
against `required_skills`, all new tests pass, and the audit's
classification-shaped rows remain untouched by automated matching.
