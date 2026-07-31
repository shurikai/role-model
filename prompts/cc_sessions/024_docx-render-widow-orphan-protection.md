# Add widow/orphan protection to section and role headers

## Problem

Section headings (SUMMARY, SKILLS, EXPERIENCE, EDUCATION) and role
headers (employer/date line + title line) can currently land at the
bottom of a page with their content pushed to the next page - e.g. a
heading isolated on page N with its first bullet/content starting on
page N+1. Currently requires manual line-break insertion to fix per
output (done by hand on the Rula v10 EDUCATION section).

## Fix

Set `keep_with_next` on paragraph_format for every paragraph in a chain
that must stay together, up through the second-to-last link - the
property only binds a paragraph to the next one, so it needs to be set
on each element up the chain, not just the first:

- Section heading paragraph -> keep_with_next = True
- Section rule-line paragraph (if a separate paragraph from the heading)
  -> keep_with_next = True
- Employer/date line paragraph -> keep_with_next = True
- Title line paragraph -> keep_with_next = True

This ensures: heading + rule + first line of content stay together, and
employer + title + first bullet of a role stay together. Do NOT apply
keep_with_next to bullet paragraphs themselves - bullets should be free
to break across a page boundary mid-list; only the header chains need
protection.

## Verification

Construct or find a fixture where a section or role header would
naturally fall near a page boundary (the Rula v10 EDUCATION case is a
known example - regenerate/reuse that exact case). Confirm the heading no
longer gets stranded from its content without any manual line-break
insertion, and confirm bullets still flow/break normally elsewhere in the
document (i.e. this didn't accidentally force whole roles onto single
pages when they shouldn't be).

## Fixture / regression case

Rula fixture (v10) in tests/fixtures - known case where EDUCATION landed
at a page boundary requiring manual correction. Post-fix regeneration
should place it correctly without intervention.
