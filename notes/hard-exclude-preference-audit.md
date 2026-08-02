# hard_exclude preference audit

Read-only audit performed 2026-08-01, alongside the change making the
anti-pattern gate non-blocking. **No preference rows were modified.** This is
input for a human decision, not a change proposal.

> **Status update.** Both defects this audit identified have since been fixed
> in `RunAntiPatternGate` — see "Resolution" at the bottom. The analysis below
> describes the code as it stood when the audit was written, and is kept
> because the reasoning about which rows are skills-shaped still holds. The
> claim that four rows "have never been able to fire" is no longer true of the
> current code.
>
> A later pass narrowed the gate again: it reads `required_skills` only, not
> `preferred_skills`. See "Second resolution" at the bottom. Wherever the text
> below says the matcher cannot see skills at all, read it as a description of
> the original code, not of today's.

## What the matcher can actually see

`RunAntiPatternGate` and `ScorePreferenceFit` both match preference labels via
`signalFields` (`internal/fitgate/scorer.go`), which returns exactly:

```go
append([]string{signals.Domain, signals.WorkType, signals.Seniority}, signals.CultureSignals...)
```

`required_skills` and `preferred_skills` are **not** in that list. Nothing in
`fitgate` matches a preference against a JD's extracted skills.

> **No longer accurate.** `RunAntiPatternGate` stopped using `signalFields`
> and now routes through `gateFieldsFor`, whose `anti_pattern` branch does read
> `required_skills` — and only that one. `preferred_skills` was briefly
> included and then deliberately removed; `signalFields` itself is unchanged,
> so this paragraph still describes `ScorePreferenceFit` correctly.

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

## False positive: any `staff`-level JD trips row 5

Found while smoke-testing the non-blocking change, and confirmed directly
against `RunAntiPatternGate`:

```
seniority="staff"     passed=false  hits=1
seniority="senior"    passed=true   hits=0
seniority="principal" passed=true   hits=0
```

`matchesSignal` compares in **both** directions — label containing field, or
field containing label. `signalFields` includes `signals.Seniority`. So the
label `IT consulting / staff augmentation model` contains the string `staff`,
and every JD extracted as staff-level matches it.

Under the old blocking behavior this meant **every staff-level job description
was hard-failed**: no technical score, no preference score, no narrative, no
resume, and the only explanation surfaced was "IT consulting / staff
augmentation model" — which the JD had nothing to do with. Staff is one of the
most common seniority levels for the roles this tool exists to target.

The bidirectional match is deliberate and documented (short canonical field
tokens vs. longer descriptive labels), so this is not a careless bug. But
substring matching in both directions across a shared field list has no way to
distinguish "staff" the seniority from "staff" inside "staff augmentation",
and it will keep producing collisions like this.

Not fixed here — this session deliberately left `scorer.go` logic alone, and
the gate is no longer load-bearing, so the false positive is now cosmetic
rather than destructive. Worth tracking separately.

## Recommendation

Change nothing yet. The useful conclusion is not "delete these rows" — it is
that **half the hard-exclude list has never been able to fire**, which is a
much better explanation for the gate's poor track record than the gate logic
itself. That is worth knowing before investing in either the skills-matching
work or the retirement decision.

## Resolution

Both defects are fixed. `RunAntiPatternGate` now routes each preference to the
signal fields its `preference_type` names, via `gateFieldsFor`, and matches on
word boundaries via `containsPhrase`. Covered by `internal/fitgate/scorer_test.go`.

**Word-boundary matching alone did not fix the staff collision, contrary to
the plan this work started from.** The assumption was that `"staff"` matching
inside `"staff augmentation"` was a character-level accident that tokenizing
would eliminate. It isn't: `"staff"` is a genuine whole word in that label.
Tokenizing changes nothing about that pair, and a test written to the original
plan fails. Verified before changing approach.

The real defect was comparing every label against every field. `"staff"` means
a seniority level in one place and part of a business-model phrase in the
other; no lexical rule can separate those, because structurally the false
positive is identical to the legitimate `domain: "defense"` → `"defense /
aerospace"` match. Routing separates them by asking which field a preference is
*about*. Seniority is now matched by nothing at all, since no preference type
describes a seniority level.

Skills reach only the `anti_pattern` branch, which is where all four
skills-shaped rows live. Domain, culture, and work_type excludes are not
compared against a tech stack. Row 5 (`IT consulting / staff augmentation
model`) is classification-shaped but shares the `anti_pattern` type, so it is
now compared against skills too — harmless in practice, since no extracted
skill will contain that phrase or be contained by it, but it is the one place
where routing by type is coarser than routing by intent.

Still open: the retirement question for rows 5–7, and row 8's
miscategorization. `work_type` can only ever be remote/hybrid/onsite/unknown,
so `"pure frontend"` stored under that type still cannot fire — routing makes
this clearer rather than fixing it. Rewording labels or recategorizing rows is
a data change and was deliberately left alone.

## Second resolution: the gate reads required skills only

Letting the `anti_pattern` branch see both skills arrays produced a false
positive of its own, and row 1 is the row that produced it.

On a Principal Java Engineer JD, `Angular` appeared exactly once — inside a
nice-to-have bullet, "exposure to front-end technologies such as React or
Angular" — which extraction correctly placed in `preferred_skills`. That
single optional mention tripped `Angular as co-equal frontend requirement`,
and the narrative then described Angular as a co-equal frontend requirement,
which is the label's wording rather than anything the JD said.

`gateFieldsFor`'s `anti_pattern` branch now reads `required_skills` only. A
hard exclude is a claim about what a job actually demands, so it should fire
on a requirement and not on a nice-to-have. `preferred_skills` still feeds
`ScoreTechnicalFit` and `ScorePreferenceFit` normally — the narrowing is
scoped to the gate.

This is the same shape of defect as the staff collision, one level up. There
the fix was routing by *which field* a preference is about; here it is routing
by *how strongly the JD asserts* the field. Both are cases where the matcher
had access to more text than the question warranted.

Consequence for the table above: rows 1–4 fire against required skills, and
no longer against optional ones. Row 1's entry ("No — skills-shaped") is
doubly out of date — it can fire, but only on a genuine requirement.

Extraction also changed alongside this. Interchangeable alternatives now
arrive as one entry joined with `" | "` ("Spring Boot | Quarkus | Micronaut |
Vert.x") instead of one entry each, so `gateFieldsFor` splits them back apart
before matching — an exclude still reaches a single option buried in a group.
