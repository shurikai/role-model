# Enforce a length/page budget on generated resumes

## Problem

2a currently generates bullets proportional to how much background data
exists per role, with no target length. Confirmed case: Rula resume ran
4 pages for a Senior-level IC role — way beyond norm (1-2 pages).

## Fix

Add a target-length constraint to the 2a prompt, informed by seniority
level (e.g. Senior/Staff IC -> 1-2 pages -> roughly N total bullets across
all roles). Bullets should be prioritized by relevance to the JD signals
from Stage 1, not included exhaustively — older/less-relevant roles get
fewer, tighter bullets; recent/most-relevant roles get more room.

Rough mechanism: convert target page count to an approximate total bullet
budget (e.g. ~12-15 bullets total for 1-2 pages at current formatting
density), and instruct the model to allocate that budget across roles by
relevance rather than emitting everything available per role.

## Open question before implementing

Should the budget be a hard bullet-count cap passed into the prompt, or a
soft instruction ("prioritize the N most relevant achievements")? A hard
cap is more reliable but risks cutting something important if relevance
scoring is off; worth trying soft first and measuring actual output length
before adding a hard constraint.

## Fixture / regression case

Same Rula fixture — post-fix regeneration should land at 1-2 pages with
older/less-relevant roles (Lockheed sub-roles, AEMWAS) getting tighter
bullet counts than Disney/Daugherty.
