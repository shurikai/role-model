# hard_exclude preference audit

Read-only audit performed 2026-08-01, alongside the change making the
anti-pattern gate non-blocking. **No preference rows were modified.** This is
input for a human decision, not a change proposal.

## What the matcher can actually see

`RunAntiPatternGate` and `ScorePreferenceFit` both match preference labels via
`signalFields` (`internal/fitgate/scorer.go`), which returns exactly:

```go
append([]string{signals.Domain, signals.WorkType, signals.Seniority}, signals.CultureSignals...)
```

`required_skills` and `preferred_skills` are **not** in that list. Nothing in
`fitgate` matches a preference against a JD's extracted skills.

This matters, because the session spec assumed the opposite — that a
primary-language exclude such as Ruby/Node/Python "*is* reliably
machine-matchable against `required_skills`". It is not, and never has been.
Four of the eight rows below were written expecting a mechanism that does not
exist.

Also worth noting: `domain`, `work_type`, and `seniority` are closed enums.
`work_type` is only ever `remote`/`hybrid`/`onsite`/`unknown`, so a label
stored under `preference_type = 'work_type'` can only ever match if it happens
to appear in a free-text `culture_signals` entry instead.

## The eight rows

| # | type | label | can it fire today? |
|---|------|-------|--------------------|
| 1 | anti_pattern | Angular as co-equal frontend requirement | No — skills-shaped |
| 2 | anti_pattern | expert Python as primary requirement | No — skills-shaped |
| 3 | anti_pattern | TypeScript / Node.js as primary language | No — skills-shaped |
| 4 | anti_pattern | production LLM / AI engineering as hard requirement | No — skills-shaped |
| 5 | anti_pattern | IT consulting / staff augmentation model | Only via `culture_signals` |
| 6 | culture | Big Four consulting culture | Only via `culture_signals` |
| 7 | domain | defense / aerospace | **Yes** — matches `domain: "defense"` |
| 8 | work_type | pure frontend | Effectively no — see enum note above |

### Group A — skills criteria that silently do nothing (1–4)

These describe what a role is built on, which is a genuinely machine-matchable
question — but against `required_skills`, which the matcher never consults.
They are not redundant with `screening_summary`; a screening summary should not
be judging whether Python is the primary language, that is a technical-fit
question.

These are the strongest candidates for real work, and the work is **not** more
preference rows. Either extend `signalFields` to include the skills arrays, or
score primary-language exclusions inside `ScoreTechnicalFit` where the skills
already are. Neither is in scope for this session.

Note that extending `signalFields` naively would also change
`ScorePreferenceFit` behavior for every positive and negative preference, since
both call it. That is a deliberate design decision, not a one-line change.

### Group B — human-judgment criteria now covered by screening_summary (5, 6, 7)

Consulting-shop culture, staff-augmentation business model, and defense/
aerospace industry are exactly the "read it and know" criteria that
`screening_summary.industry` and `screening_summary.other_flags` were added to
surface. Row 7 is the only one of the eight that reliably fires, and it is also
the one `screening_summary.industry` captures most cleanly.

Candidates for retirement once you trust the screening summary in practice —
but retire them on evidence, after watching a few real JDs produce summaries,
not on this table.

### Group C — miscategorized (8)

"pure frontend" is stored as `work_type`, but `work_type` cannot express it.
It is a statement about role shape, which is either a skills question (Group A)
or a screening-summary observation. Whichever it becomes, it is not doing
anything where it currently sits.

## Recommendation

Change nothing yet. The useful conclusion is not "delete these rows" — it is
that **half the hard-exclude list has never been able to fire**, which is a
much better explanation for the gate's poor track record than the gate logic
itself. That is worth knowing before investing in either the skills-matching
work or the retirement decision.
