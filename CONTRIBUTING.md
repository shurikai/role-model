# Contributing

This is a personal project that happens to be public. Bug reports and questions
are welcome; large unsolicited pull requests probably are not, because the
design has opinions and most of them are written down.

Read [`CLAUDE.md`](./CLAUDE.md) before changing anything substantial. It is the
conventions document — stack, architecture, and a **Do Not** list — and most of
its rules exist because the alternative was tried and cost something. Where a
rule looks arbitrary, the commit that introduced it usually explains why.

## Getting set up

```bash
docker compose up --build          # the whole stack
```

or, without containers:

```bash
make setup && make seed-sample && make dev
```

See the README for both paths in full.

## Before you push

```bash
make fmt          # gofmt, prettier, ruff
make test-all     # Go unit + integration, vitest, pytest
```

`make test` alone is Go unit tests only — roughly 40% of the suite. `test-all`
needs a database; `make test-race` additionally needs a C toolchain, and CI
runs it either way.

## Things that will get a patch sent back

**A new Go dependency without a reason a reviewer would agree with.** This is
about `go.mod` specifically: it ships inside a single-binary server that people
self-host. `frontend/` and `docx-renderer/` follow their own ecosystems' normal
practice and need no such justification — that distinction is in CLAUDE.md
under **Dependencies**, and it has been misread in the wrong direction before.

**A migration numbered at or below the current maximum.** golang-migrate
applies only versions above the database's current one, so a lower-numbered
file is skipped forever while `migrate up` reports "no change". CI checks this;
`make check-migrations` runs the same check locally.

**A test that cannot fail.** Twice in this repo a green test was asserting
nothing — a parser that returned no rows, a fixture with none of the data the
assertion was about (#93). Plant the defect your test exists to catch and watch
it go red before you trust it. If it reads a fixture off disk, use `-count=1`;
the Go test cache does not track file contents.

**A closed vocabulary beside a free-text field that already answers the
question.** Several enums have been deleted for this. Levels, proficiencies and
resume sections are user-owned rows, not Go constants.

**Prompt edits invented rather than reasoned.** Prompts live in
`internal/generation/prompts/` and are identified by the git blob hash of their
content, recorded per generation. Commit prompt changes before generating
anything you want to trace back.

## Commit messages

Long, and about *why*. The repository's history is its design record; a diff
shows what changed and almost never why it had to. Look at recent commits for
the register.

## Issues

`gh` is the tracker. Existing labels: `stage-0`, `stage-1`, `stage-2`,
`renderer`, `frontend`, `infra`, `backlog`, `bug`, `enhancement`. Prefer one of
those to a new one.

## What is most useful right now

The gaps in the README's **Known gaps** section, roughly in that order. A UI
for skills and preferences is the one that unblocks the most — the fit gate
scores against preferences, and there is a full API behind nothing.
