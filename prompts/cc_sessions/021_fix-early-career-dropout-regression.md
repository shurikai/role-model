# Fix: early-career sub-role dropped entirely by length-budget logic

## Problem

Verified regression in rula-senior-software-engineer-patient-onboarding-v5.docx,
the first real regeneration after #017/#018/#019. The Lockheed Martin
"Software Engineer, Jun 1999 - Jan 2003" sub-role was omitted entirely -
no title, no dates, no bullets - creating an unexplained ~3.5 year gap at
the start of the career. Violates the canonical hard rule that employer/
date-range coverage must never be dropped.

#019's own verification claimed to have caught and guarded against a
similar edge case ("ambiguous wording nearly dropped an entire
early-career position"). That guard either didn't generalize to this case,
or was verified against a different scenario than what actually happened
here.

## Likely root cause (needs confirmation)

019's relevance-based allocation may be operating on "roles/sub-roles" as
the unit of what can be cut, not just "bullets within a role." If a
low-relevance-scored sub-role's bullet allocation rounds down to zero, the
whole sub-role - including its title and date range - may be getting
dropped from the output rather than just having zero bullets rendered
under an otherwise-present title/date line.

## Fix

1. Confirm the root cause above by tracing 2a's allocation logic for this
   specific case.
2. However the budget is enforced, the constraint must be: every
   employer/sub-role's title + date range is always emitted, regardless
   of relevance score. Only bullet content within a role can be trimmed,
   down to a minimum of at least one bullet per role/sub-role if any
   exist in the source data - title/dates alone with zero bullets is
   probably fine for a low-relevance role, but the entity itself must
   never disappear.
3. Add this as an explicit hard constraint in the 2a prompt (not just an
   instruction to "avoid gaps" - the ambiguous wording that reportedly let
   the original edge case through).

## Verification

Must be checked against a REAL regeneration (not just a synthetic/unit
test case) before closing, given the previous "fix" was verified in a way
that didn't catch this. Regenerate the Rula fixture and manually confirm
every canonical employer/sub-role and its full date range appears in
output, cross-checked against canonical-context.md's hard employment
timeline rule.

## Relationship to #31 (length budget)

Entangled: v5's bullet count landed near target partly *because* a whole
sub-role vanished, not purely through bullet-level trimming. Don't close
#31 until this is fixed and re-verified together - a version that hits the
bullet-count target by dropping content is not actually a passing case.

## Fixture / regression case

rula-senior-software-engineer-patient-onboarding-v5.docx (regression case)
and v1 (original bug case) in tests/fixtures. Post-fix regeneration should
preserve full Lockheed timeline (both sub-roles) while still landing near
the length target.
