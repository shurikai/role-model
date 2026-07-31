# Refactor Python docx renderer to match canonical formatting spec

## Problem

The Python docx-render code was written intentionally rough to ship an MVP
fast. Comparing its output (rula-senior-software-engineer-patient-onboarding-v5.docx)
against resumes generated via the Node.js/docx pipeline used in ad hoc
per-JD chat sessions, the Python renderer is visibly less compact:

- Role headers render as 3 stacked lines (company, title, date each on
  their own paragraph) instead of 2 (company+date on one line via a right
  tab stop, title on the next).
- Headings likely use library/Word default heading styles, which carry
  built-in spacing well beyond what's needed, rather than explicit
  paragraph-level spacing overrides.
- Contact block renders as 4 stacked lines instead of one compact
  centered line.

This inflates page count independent of content volume - part of why
#31/#021's bullet-count-based length work hasn't translated cleanly into
actual page-count results.

## Goal

Refactor the Python renderer to match the "Formatting Standards" section
of canonical-context.md exactly, AND converge its output with the Node.js/
docx pipeline used in chat-based resume generation, so that either
pipeline produces structurally and visually equivalent output. No more
guessing which one is more "correct" or maintaining two different
interpretations of the same spec.

## Scope

Layout/rendering only. Do NOT touch:
- What data flows into the template (bullet selection, summary content,
  skills selection) - that's #021/#019's territory.
- Anything upstream of "structured resume data -> docx bytes."

If this session finds itself wanting to change bullet count or content to
"make it fit," that's scope creep - stop and flag it instead.

## Concrete changes (per canonical-context.md Formatting Standards)

- Font: Arial throughout.
- Accent color: #280137 for section headings and rule lines.
- Section headings: spacing before 320 / after 20; rule line after with
  before 0 / after 120.
- Employer line: company (bold) + tab + tenure (gray), single line via
  right tab stop, NOT a stacked heading. Tab stop position: 10240 twips
  (CONTENT_WIDTH = PAGE_WIDTH_DXA 12240 - MARGIN_DXA 1080*2). Do not use
  TabStopType.MAX or any margin-derived default - hardcode 10240.
  Split into three separate TextRun objects (company / tab / tenure) per
  spec - do not embed a tab character in a single string.
- Title line: italic, spacing before 0 / after 60, own paragraph
  immediately below employer line (2 lines total per role header, not 3).
- Bullet indentation: left 480, hanging 280. Bullet spacing before 20 /
  after 20.
- Page margins: 1080 all sides (top/right/bottom/left).
- Contact block: single compact centered line (or as few lines as
  reasonably readable), not one line per contact field.
- No use of Word built-in Heading 1/2/3 styles for section headers or
  role headers - use explicit paragraph-level spacing overrides matching
  the values above, the same way the Node.js/docx reference implementation
  does.

## Verification

1. Take the same structured resume data (e.g. the Rula fixture content)
   and render it through both the Python renderer and the Node.js/docx
   pipeline. Page count, line count, and visual layout should match
   closely - not pixel-identical (different libraries), but structurally
   equivalent: same number of lines per role header, same heading spacing
   behavior, same overall page count for identical content.
2. Confirm bullet count and content are UNCHANGED before/after this
   session - only layout should differ from the pre-refactor version.
3. Re-render the v5 Rula fixture (once #021's content fix lands) through
   the refactored Python renderer and visually confirm it now reads as
   compact/clean as the Node.js/docx output for equivalent content.

## Fixture / regression case

Rula fixture in tests/fixtures - same structured content rendered via
both pipelines, kept as a pair to catch future drift between them.

## Note

Once this lands, canonical-context.md's Formatting Standards section
becomes the single source of truth both pipelines implement - if either
pipeline's output ever needs to change, the spec doc should be updated
first, then both implementations brought back into line, rather than one
silently drifting ahead of the other.
