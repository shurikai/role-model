# Ensure JD-required confirmed skills are surfaced in generation

## Problem

2a (bullets + skills generation) doesn't reliably surface every confirmed
skill from background data that's relevant to the JD. Confirmed case: Rula
resume omitted AI-assisted dev tooling (Claude Code/Copilot/Cursor) despite
canonical context confirming daily use + an architected pipeline, and the
JD listing it as a required qualification.

This isn't a fabrication risk (nothing false was added) — it's a recall
failure. The model had the fact and didn't use it.

## Fix

Add an explicit cross-check step to the 2a prompt: for each required/
preferred qualification identified in Stage 1's JD-signals extraction,
confirm whether background data supports it, and if so, ensure it appears
in either Skills or a bullet. Don't rely on the model to spontaneously
surface everything relevant — give it the JD requirement list back as an
explicit checklist input, not just loose context.

## Open question before implementing

Should this checklist step be inline in the same 2a prompt (single call,
extra instruction), or a distinct verification step after 2a drafts
(2a -> checklist against Stage 1 signals -> patch/append)? Inline is
cheaper; a separate step is more reliable and gives a natural home for a
future "did we miss anything required" check without waiting for the full
three-pass reviewer.

## Fixture / regression case

Same Rula fixture in tests/fixtures — post-fix regeneration should include
AI-tooling in Skills or a bullet given the JD's explicit requirement.
