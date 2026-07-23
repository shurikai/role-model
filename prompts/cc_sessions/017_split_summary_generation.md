# Split summary generation from bullet/skills generation

## Problem

Generation is currently one LLM call: JD signals + full background data ->
entire resume (bullets, summary, skills) in a single pass. This lets the
summary draft reach past whatever bullets actually got selected and state
things not grounded in them — confirmed case: a generated summary claimed
"on-call owner" with no corresponding bullet anywhere in the output.

## Fix

Split the second pipeline call into two sequential calls:

1. **2a — Experience + Skills:** signals + full background data -> bullets
   and skills only (unchanged inputs, narrowed output).
2. **2b — Summary:** signals + the output of 2a ONLY (not raw background
   data) -> summary paragraph.

2b must not receive the full background corpus. Its only source material
is what 2a already produced. This makes ungrounded claims structurally
unreachable rather than relying on a prompt instruction to self-police.

## Open question before implementing

Does 2b need any scalar background fields (total years of experience, etc.)
that don't naturally live in bullets/skills? If so, those need to be passed
explicitly and narrowly — not the full corpus — to preserve the constraint.

## Why now

This is the cheapest structural fix and it creates the seam the three-pass
reviewer will eventually plug into (2a output -> verify -> 2b, or a
standalone check between 2a and 2b later). Doing the split now means the
reviewer work later is additive, not a rearchitecture.

## Fixture / regression case

Chestnut_Jason resume generated for Rula (Senior SWE, Patient Onboarding),
[date]. JD + output on file. Once fixed, same JD/data should regenerate
without an on-call claim unless a bullet supports it.
