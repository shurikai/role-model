# CC Session: Canonical Vocabulary Audit (Documented Skills vs. Seeded Rows)

## Context

`JFrog Artifactory` was flagged by three consecutive fit reports as a
technical gap despite being a confirmed, real skill (Southwest/Daugherty
engagement). Investigation confirmed it was a **data-gap, not a
vocabulary-gap**: no `skills`/`tags` row exists for it at all. It was only
discovered because a JD happened to name it explicitly — the only
detection mechanism that exists today is coincidence. Any other
documented-but-unseeded skill is sitting in the same blind spot until a
JD happens to probe it, which is an unacceptable discovery mechanism once
this moves from single-user (Jason, who can cross-reference from memory)
to testers who have no way to know what should be there.

This session is a one-time audit — diff the documented canonical skill
set against what's actually seeded — plus a small reusable script so the
same check can be re-run later rather than repeating this session by hand
next time.

**Important scope note:** the real seed data lives in the private
`role-model-seed` repo (SQL seed files, stable UUIDs, upsert semantics),
**not** in this repo. `database/sample/` and `database/sample-clinical/`
here are demo/fixture datasets, not Jason's real data — do not audit
against those. This session needs either:
(a) `DATABASE_URL` pointed at the real dev/prod database, or
(b) the `role-model-seed` repo checked out alongside this one.
Confirm which is available at session start before proceeding — do not
assume and do not fall back to the sample data as a substitute.

**Source-document precondition:** this session reads `canonical-context.md`
from `notes/canonical-context.md` in this repo. If that file is not
present at session start, stop and tell Jason — do not proceed on a
half-audit, and do not reconstruct the document from memory, from prior
fit reports, or from anything else in the repo. Jason is responsible for
committing an up-to-date copy before this session runs; that commit is a
precondition of this prompt, not a task within it.

It is not a scheduled job or a CI check in this session; that's a future
decision once this proves useful once.

Stack: Go, new `cmd/vocabaudit/main.go`, reads existing `tags`/`skills`
schema (`migrations/001_initial_schema.up.sql`,
`migrations/005_add_skills_preferences.up.sql`) via `internal/db`
queries — check whether `ListActiveSkillsByUser` /
`ListActiveSkillTagNamesByUser` in `internal/db/queries/skills.sql`
already return what's needed (tag name + aliases + category) before
writing a new query; only add one if the existing ones don't cover it.

---

## Session Start (always)

```bash
git pull
gh issue list --state open
test -f notes/canonical-context.md && echo "source doc present" || echo "STOP: notes/canonical-context.md missing"
```

Confirm with Jason which of the two options above (real `DATABASE_URL`
vs. `role-model-seed` checkout) is available before Task 1. If
`notes/canonical-context.md` is missing, stop here per the precondition
above rather than substituting anything else.

## Task 1: Extract the documented skill list

From `notes/canonical-context.md` (read directly from the repo checkout —
do not ask Jason to paste it, do not reconstruct it from memory), extract
the explicit skill/tool names Jason has already enumerated — the
"Confirmed skills" section is the primary source, but also scan the
"Canonical Facts" section for named technologies (Java, Go, Groovy,
C#/.NET, Python, Kafka, Cassandra, etc.) and the employer-specific
mentions (e.g. "MAK Technologies: real-time Kafka integration via C++
plugins" names both Kafka and C++).

Do **not** use an LLM call to infer or expand this list — this is a
literal extraction of what's already explicitly named in the text, not a
semantic reading. If a term's presence is ambiguous (e.g. "AI tooling:
Claude Code, GitHub Copilot, Cursor" — are these meant as `skills` rows or
just narrative context?), list it separately as "ambiguous, needs Jason's
call" rather than guessing either way.

Output: a flat list, each entry noting which section/employer it came
from (for traceability back to the source doc when reviewing results).
Also record the file's git commit hash (`git log -1 --format=%H --
notes/canonical-context.md`) in the report header, so a stale-doc concern
during review can be checked against when the source was last updated.

## Task 2: Query seeded skills for the user

Using whichever data source was confirmed at session start, list every
`tags` row (with `aliases`, `category`) that has a corresponding active
`skills` row (`is_active = true`) for Jason's user id. Reuse
`ListActiveSkillsByUser`/`ListActiveSkillTagNamesByUser` if they return
enough (name, aliases, category) — check their actual `SELECT` columns
before assuming, per session 018's precedent of skill-retrieval
completeness issues. If they don't return aliases, that's itself worth
noting as a finding, not just working around it silently.

## Task 3: Diff and report

For each Task 1 entry, check for a match (case-insensitive) against
either `tags.name` or any element of `tags.aliases` from Task 2.

Produce a markdown report (`notes/vocab-audit-YYYY-MM-DD.md` or similar,
Jason to confirm exact naming) with two clearly separated sections:

- **Documented but not seeded** (the urgent list — this is the JFrog
  pattern). For each: the skill name, which part of
  `notes/canonical-context.md` it came from, and — if the source text
  names an employer/engagement for it — that employer, since it's needed
  for evidence linkage in Task 4.
- **Seeded but not documented** (lower priority, informational). For
  each: the tag name, category, and a note that this may be legitimate
  and not require any action.

Do not merge these into one undifferentiated list — they call for
different responses and conflating them would understate how urgent the
first category is.

## Task 4: Draft (not apply) seed statements for confirmed gaps

For entries in the "documented but not seeded" list that have a clear
employer/evidence source named in `notes/canonical-context.md`, draft a
seed `INSERT` statement following the existing pattern in
`database/sample/007_skills_preferences.sql` (stable UUID, matches the
`role-model-seed` repo's upsert-semantics convention if that repo is
available to reference — check its existing file structure before
guessing the format).

For entries with no clear evidence source in the text, do not draft a
statement — list them as "needs evidence source from Jason" instead of
guessing an employer/position to attach them to. A wrong evidence link is
worse than no seed row, since it would misrepresent provenance.

Output these as a single reviewable `.sql` file, not applied — Jason
reviews and runs it manually, or feeds it into `role-model-seed`'s normal
process.

## Task 5: The reusable script

Package the Task 1–3 logic (not Task 4's draft-generation, which is
inherently judgment-heavy) as `cmd/vocabaudit/main.go`: flags for a path
to the canonical text file (default `notes/canonical-context.md`) and a
user id/email, connects to `DATABASE_URL`, prints the same two-section
report to stdout. Mirror `cmd/intakerun/main.go`'s structure and
doc-comment style (why it exists, why it's not wired to the server).
Have the script fail loudly (non-zero exit, clear message) if the source
file at the given path doesn't exist, rather than silently producing an
empty "documented" list.

Do not attempt to make Task 1's extraction fully automatic/parseable in
this script — the "ambiguous, needs Jason's call" case from Task 1 means
this will likely always want a human skimming the source doc alongside
the tool's output, at least until the canonical doc's format is more
structured than free-form markdown. Note this limitation in the script's
doc comment rather than over-engineering a parser for prose that wasn't
designed to be machine-read.

---

## Do NOT

- Do not proceed if `notes/canonical-context.md` is missing from the repo
  at session start — stop and tell Jason, per the precondition above.
- Do not ask Jason to paste `canonical-context.md` into the session, and
  do not reconstruct its contents from memory, prior fit reports, or
  anything else in the repo — the repo copy is the only valid source.
- Do not audit against `database/sample/` or `database/sample-clinical/`
  — those are demo fixtures, not real data. Confirm the real source per
  the Session Start note before running anything.
- Do not auto-write to the seed repo, run migrations, or otherwise apply
  any change unattended — Task 4's output is a reviewable draft only.
- Do not use an LLM call to expand or infer the canonical skill list in
  Task 1 — literal extraction from the supplied text only, ambiguous
  cases flagged for a human decision.
- Do not merge the "documented but not seeded" and "seeded but not
  documented" findings into one list.
- Do not guess an evidence employer/position for a gap that doesn't
  clearly name one in the source text.
- Do not build this as a scheduled job, CI check, or server-wired
  endpoint — `cmd/` only, run manually, same as `cmd/intakerun`.

---

## Verification Steps

```bash
go build ./...
go run ./cmd/vocabaudit --file notes/canonical-context.md --email jason.chestnut@gmail.com
```

Session is complete when: `notes/canonical-context.md` was confirmed
present and read directly from the repo (not pasted, not reconstructed),
the audit report exists with both sections populated and clearly
separated, the draft seed `.sql` file covers every confirmed-gap entry
that had a named evidence source, every ambiguous or evidence-less entry
is explicitly listed rather than silently dropped or guessed,
`cmd/vocabaudit` builds and runs against a real database connection and
fails loudly if the source file is missing, and nothing was written to
the database or the seed repo without Jason's manual review.
