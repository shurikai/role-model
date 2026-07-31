# Fix deprecated field names in jd_signals integration test fixture

## Problem

The integration test's jd_signals fixture references priority_skills and
domain_vocabulary, which no longer match the current schema. Confirmed
pre-existing and unrelated to #017 (split-summary-generation) — fails the
same way on unmodified main. Discovered while verifying #017; a temporary
local patch was reverted rather than folded into that session's scope.

## Fix

1. Check current jd_signals schema for the correct field names that
   replaced priority_skills and domain_vocabulary.
2. Update the fixture to use current field names.
3. Grep for any other references to the deprecated names (fixtures, test
   helpers, docs) to make sure this isn't a wider drift issue than just
   the one fixture.
4. Confirm the integration test passes on main after the fixture fix, with
   no other changes needed — if the test still fails, that's a real schema
   bug, not just a stale fixture, and should be re-scoped.

## Before starting

Run `gh issue list --state open` to check whether this overlaps with the
pre-existing "prompt eval strategy" backlog issue (#9-13 range) before
filing/working — this could be the same underlying drift.

## Fixture / regression case

N/A — this session fixes a fixture itself. Once resolved, the integration
test suite passing cleanly is the verification.
