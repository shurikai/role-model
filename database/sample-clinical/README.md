# Clinical sample dataset

Fictional. Priya Raghunathan does not exist and neither does any employer here.

This dataset is the acceptance test for the career-neutrality work (#78). The
question it answers is not "can the pipeline store a nurse" — the data model
was always close to neutral — but **"can someone whose career is nothing like
the one this was built around get from a pile of notes to a resume, without
anyone changing code for them?"**

## It was produced by the intake, not written by hand

That distinction is the whole point. A hand-authored second seed would prove the
pipeline is career-neutral and prove nothing about the low-friction path, which
is the half that actually decides whether a non-engineer can use this.

`career-notes.txt` is a 640-word first-person career narrative — the kind of
thing someone writes for themselves, not a resume. It was pasted into an account
holding **nothing but a user row and the shipped neutral vocabularies**: no
employers, no positions, no tags, no categories. Then:

```
DATABASE_URL=... ANTHROPIC_API_KEY=... \
  go run ./cmd/intakerun -file database/sample-clinical/career-notes.txt \
    -email priya@example.com -user 5b000000-0000-0000-0000-000000000001
```

`stage0_career_extraction.tmpl` read it, `internal/intake` planned and staged 25
drafts, and the resolver wrote them in dependency order inside one transaction:

```
staged 25 drafts {"contribution":7,"employer":3,"position":4,"skill":11} (11 flagged for review)
resolved 25 drafts into rows
```

The eleven flags were all `new_categories` — `Certifications`,
`Health Information Systems`, `Clinical Specialties`, `Languages` and others,
each correctly surfaced because `ResolveOrCreateTag` deliberately creates
categories with no competency vocabulary rather than guessing it.

## What a human did afterwards, and why

The draft/approve loop assumes a person reviews. This is what that person did,
recorded because the gaps it papers over are real and some of them are bugs.

**Merged eight categories into four.** The extractor proposed thirteen, several
of them one item each and near-duplicates: `Quality Improvement` /
`Quality and Safety` / `Process Improvement`, and
`Education and Training` / `Implementation and Training` / `Patient Education`.
The prompt asks for "as few as the material supports" and says "a category you
invent for one item is noise", and it did not follow that well. **This is a
prompt defect, not a review chore.**

**Wrote the category vocabulary.** `ResolveOrCreateTag` creates categories with
empty `aliases` on purpose — guessing competency terms from a tag name produces
exactly the over-broad aliases the seed tests reject. So the review has to
supply them, and the flags are what say so. `Clinical Specialties` and
`Languages` are still empty on purpose: bare "clinical" or "languages" would
grant a whole category to any posting that used the word.

**Replaced the career ladder.** The extractor wrote free text into
`industry_level` — "Staff level", "Coordinator level", "Charge/supervisory
level" — because **the career-extraction prompt does not render the account's
`career_levels` the way `jd_extraction.tmpl` renders them.** Those values match
nothing on the ladder, so the length and framing lookup falls through to the
fallback rung every time. The dataset now carries a six-rung nursing ladder
(`new grad` → `director`) with `source = 'inferred'`, and the positions are
retyped onto it. **This is a bug in the prompt and it is the highest-value
follow-up here.**

**Added identity, education, credentials and preferences by hand.** The
extraction prompt asks for employers, positions, contributions and skills only.
Everything else in `career-notes.txt` was dropped — including the last
paragraph, which is her stating in plain words what she wants next and what she
will not take. **A person saying what they want is the single most quotable
thing in a career narrative, and the extractor never asks for it.** The
preferences here were read straight off that paragraph.

The extractor also filed ACLS, BLS and the ANCC certification as *skills*
rather than *credentials*, which is defensible — they are things she can do —
but means the CREDENTIALS section of a rendered resume would be empty for a
licensed professional. They are duplicated as credentials here.

## What it proves

`internal/fitgate/testdata/profile-clinical.json` is generated from this
dataset, and `internal/fitgate/testdata/cases/clinical/` holds two paired job
descriptions. The eval harness runs both profiles:

```
CASE                          TECH PREF m/g/c  GATE  VERDICT
clinical-dealbreaker-fires    80.0     0/6/0  FAIL  ok
clinical-strong-match         83.3     5/1/0  pass  ok
```

A matching ambulatory quality-improvement posting scores 83.3 with five
preference matches. An inpatient night-shift posting correctly trips the hard
gate — through `preferences.aliases`, because the posting says "three
twelve-hour night shifts" and she said "inpatient nights".

**No code changed to make either happen.**

## Loading it

```
make db-up && make migrate-up
for f in database/sample-clinical/0*.sql; do psql "$DATABASE_URL" -f "$f"; done
```

Every file is upsert-safe and re-runnable. UUIDs all begin with `5b`, so this
dataset cannot collide with the software sample (`5a`, `5c`, `57`) or a real
seed set in the same database.
