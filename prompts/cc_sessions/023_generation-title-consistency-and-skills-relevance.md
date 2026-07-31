# Fix: header title is authoritative, summary must match; extend relevance filtering to skills

## Problem 1 — Title/summary inconsistency

Header title (e.g. "Staff Backend Engineer") is a fixed personal-branding
field, but the summary is drafted independently by the LLM and free to
choose its own leveling language per JD. Confirmed case: Rula resume
header says "Staff Backend Engineer," summary opens "Senior Software
Engineer with 27 years..." - same document, two different levels stated.

## Fix 1

Decision: header title is authoritative. Pass the header title string
into the summary-generation step (2b, per #017's split) as an explicit
input, and instruct it to open with that exact title verbatim rather than
independently deriving or restating a level. Same pattern as #017: thread
a fact through as an explicit input rather than letting two generation
steps independently decide the same thing.

Note: this means the summary should ALWAYS reflect Jason's actual target
level (Staff), even when applying against a JD posted at a different
level (e.g. Senior). This is intentional positioning, not a bug to
correct toward matching the JD title.

## Problem 2 — Skills list not relevance-filtered

#019 scoped its JD-relevance/budget logic to bullet selection only.
Skills selection was never touched, so thin/irrelevant skills that should
have been filtered out re-surface inconsistently. Confirmed case: GCP
(a single Daugherty cohort prototype, not production depth) reappeared in
v10's skills list alongside AWS at equal visual weight, along with
DDS/HLA/DIS/JavaScript - all previously trimmed for relevance in v5.

## Fix 2

Extend the same JD-relevance filtering logic used for bullet selection in
2a to skills selection, so thin/tangential/irrelevant skills get the same
treatment bullets already get.

This is a stopgap, not a full fix - the underlying gap is that skills
currently carry no confidence/depth signal distinct from bullets (a
one-off cohort prototype and years of production use both just render as
"GCP" / "AWS" with no way for generation to tell them apart). The real
fix is the planned skill_provenance schema work (skills table with
proficiency + provenance junction to contributions) already on the
roadmap. Flag this session's fix explicitly as interim, and note the
skill_provenance schema work as the permanent resolution once scheduled.

## Verification

Regenerate the Rula fixture. Confirm:
- Summary's opening title matches header title exactly.
- GCP does not appear unless a JD specifically calls for GCP experience
  (at which point it should probably be flagged as a thin gap rather than
  claimed as a skill, per canonical-context.md's honest-gap-representation
  rule - worth checking whether relevance filtering alone is sufficient or
  whether thin skills need a distinct "gap, don't claim" treatment rather
  than pure inclusion/exclusion).

## Fixture / regression case

Rula fixture in tests/fixtures - regenerate and diff title consistency and
skills list against v10.
