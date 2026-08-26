# Canonical Vocabulary Audit — 2026-08-25

Documented canonical skills vs. seeded rows, for
`jason.chestnut@gmail.com` (`a0000000-0000-0000-0000-000000000001`).

| | |
|---|---|
| Source document | `notes/canonical-context.md` |
| Source commit | `66d4ed08075a4cea8c4b344a0b6e3e5e27112a5f` |
| Data source | live dev DB, `localhost:5433/role_model` |
| Seed repo | `database/seed/` @ `role-model-seed`, consulted for format only |
| Active skills at audit time | 62 |
| Active skills after remediation | 68 |

The audit itself was read-only. Its findings were then reviewed with Jason in
the same session, applied to the dev database, and written back to the seed
repo as **`028_vocabulary_audit_2026_08_25.sql`** — which is where the
`INSERT`s live now, and the only copy that survives a reseed. A draft
`notes/vocab-audit-2026-08-25.sql` existed in between and was deleted once 028
superseded it; two idempotent copies of the same statements is one too many.

---

## Read this first: the audit does not cover the bug that prompted it

`JFrog Artifactory` appears **nowhere in `notes/canonical-context.md`** — no
match for `jfrog`, `artifactory`, `southwest`, or `edward jones`. It is neither
documented nor seeded.

> **Correction, confirmed in session:** the engagement is **Edward Jones**, not
> Southwest. The session spec's own context paragraph said "Southwest/Daugherty";
> both are Daugherty engagements, which is how the two were crossed. The draft
> links JFrog to `c0000000-…-0000000b0001`, the Edward Jones monolith
> decomposition, whose description already carries the build-and-deploy context.
>
> This is worth noting for its own sake: the one fact the spec asserted about
> the motivating bug was wrong, and nothing in either store could have
> contradicted it, because neither store holds the skill.

So the diff this session was asked to run would not have caught it. The premise
in the session spec — "documented but not seeded" — describes a gap class that
JFrog is not in. The real class is **known to Jason but written down nowhere**,
and a document-vs-database diff cannot see it, because both sides are missing
the same entry.

That is worth stating plainly because it bounds what the reusable script can
promise. `cmd/vocabaudit` will catch drift between the canonical doc and the
DB. It will not catch a skill that never reached either one, which is what
happened here. Closing that needs a different mechanism — the canonical doc
growing an explicit per-engagement tool list, or the JFrog-style discovery
staying manual.

A second finding of the same kind, from the seed repo: `022_skills_added_outside_seed.sql`
records 17 skills that existed only in the database because they were entered
through the app, and were lost to a `make migrate-down`. Its closing line is the
same lesson from the other direction — *"the seed repo is not a backup of the
database."* Doc, DB, and seed repo are three stores that drift independently,
and this audit only compares two of them.

## Section note

The spec names a **"Confirmed skills"** section as the primary source. That
section does not exist in the document. Its sections are: Hard Rule —
Employment Timeline, Canonical Facts, Formatting Standards, Job Search
Targeting, Stage 1 / Stage 2 Pipeline, Pipeline Steps.

Extraction therefore used **Canonical Facts** as the primary source, plus the
employer mentions in Hard Rule, per the spec's fallback instruction.

## Matching method

Case-insensitive, **whole-word**, boundary = "not alphanumeric" — the same rule
`internal/fitgate` and `reconcileSkills` use, and for the same reason. Plain
substring matching was tried first and produced false "documented" hits for
`REST` and `Git` inside ordinary English. A documented term counts as seeded if
it matches `tags.name` **or** any entry in `tags.aliases` for an active skill.

---

# 1. Documented but not seeded — the urgent list

Only two unambiguous claims are genuinely missing, and neither is the JFrog
shape. **The gap has three distinct shapes, and the spec assumed one.** They
need different fixes:

| shape | meaning | visible to fit gate? |
|---|---|---|
| **A. No tag at all** | nothing in the vocabulary | no |
| **B. Tag exists, no `skills` row** | vocabulary present, claim absent | **no** |
| **C. Tag + `skills` row, `is_active = false`** | deliberately retired | no, by design |

Shape B matters more than it looks: `ListActiveSkillMatchTermsByUser` is driven
off `skills`, so a tag with no skills row contributes nothing to matching —
it is exactly as invisible as no row at all, while *looking* present to anyone
who greps `tags`.

### Shape B — tag exists, no `skills` row (actionable)

| skill | source | employer / evidence | tag_id |
|---|---|---|---|
| **Node.js** | Hard Rule — Employment Timeline ("TypeScript/Node.js consulting") | Pelotech | `90000000-…-000000000051` |
| **Bubble Tea** | Canonical Facts ("dedupe: Go CLI … Charm/Bubble Tea TUI") | dedupe (personal project) | `90000000-…-000000000053` |

Draft `INSERT`s for both are in the `.sql` file.

### Shape C — seeded, deliberately deactivated (probably correct; do not "fix" blindly)

| skill | state | reading |
|---|---|---|
| **TypeScript** | tag `90000000-…-000000000006`, skill `5e000000-…-000000000006`, `novice`, **0.3 yrs**, `is_active = false` | 0.3 years ≈ the four-week Pelotech engagement. This was seeded *accurately* and retired on purpose — TypeScript is on the hard-exclude list and Pelotech is the omit-by-default employer. |

**No draft statement is offered for TypeScript.** Reactivating it would reverse
a deliberate decision, and the doc gives two independent reasons it was made.
It appears here because it is documented and not *active*, which the diff
correctly flags — but the correct action is probably none.

Same question applies to Node.js: it is on the hard-exclude list for the same
reason. The draft mirrors TypeScript's shape (`novice`, `0.3`, `is_active =
FALSE`) rather than asserting an active claim. **Flipping that to TRUE is your
call, not the audit's.**

### Shape A — no tag at all (evidence confirmed in session 2026-08-25)

Jason supplied evidence sources for every one of these. They divide into three
outcomes.

**Resolved — drafted and dry-run clean:**

| term | evidence | outcome |
|---|---|---|
| GitHub Copilot | Disney `c0000000-…-0000000d0006` — the AI-tooling pilot, which names all three tools verbatim | new tag + active skill, mirroring Claude Code |
| Cursor | same contribution | new tag + active skill, mirroring Claude Code |

The trio is the audit's sharpest drift example: **one sentence in the doc,
three tools, one seeded row.**

**Resolved — deliberately no row:**

| term | reason |
|---|---|
| Temporal | confirmed aspirational (Role Model future plans). The doc already calls it the "intended" orchestration layer. Not a claim. |
| TypeScript | confirmed correct exactly as seeded — `novice`, 0.3 yrs, inactive. |

**Seeded after decision:**

| term | evidence | modelling call |
|---|---|---|
| Mentoring | Manifold `c0000000-…-00000eef0003` — "Mentored 5 relatively junior RBC engineers; all became independent" | Methodologies, `expert`, active. Filed there because that is where the other non-tool capabilities already live (Agile, SOA, Concurrency) |
| macOS | personal dev environment plus Manifold, Daugherty, Disney | Tools & CI/CD — **the one arbitrary call in the file.** No Platforms category exists, and creating one for a single member would need alias vocabulary of its own. No contribution link: nothing names it, so it will not appear in `v_skill_provenance`, correctly |

**Decided against, recorded so the next audit doesn't re-raise them:**

| term | why not |
|---|---|
| Customer-facing / FDE-style | closer to the "Communication" case CLAUDE.md excludes as unfalsifiable than to a nameable, checkable capability. This is the distinction that separated it from Mentoring |
| Windows desktop | C#/.NET from the same Lockheed sentence already carry the signal, and matching better on Windows desktop roles works against a profile listing C#-as-primary among its hard excludes |
| dead-reckoning | DIS alone carries the distributed-simulation signal; stays bullet material |

### The mentoring evidence — resolved, with a caveat that outlives it

The first pass found **exactly one** contribution recording mentoring, which
made `expert` the weakest assumption in the draft. Jason then enumerated the
actual history, and it maps to **eight contributions across four employers**:

**Seven links across three employers**, after Jason dropped Daugherty:

| employer | rows | recorded in the row text? |
|---|---|---|
| Lockheed Martin | `ead0008` IOS (team of 7, and the F-35 entry), `ead0005` WARSIM, `ead0004` SE Core | yes — "led a team of 7", "software lead" |
| Lockheed Martin | `ead0003` C4I Adapter | **no** → fixed in section 6 |
| Lockheed Martin | `ead0009` UK Army Scout | yes, and **refined** in section 6 |
| Dignitas | `ace0001` IAAR | yes — "leading a team of 3 engineers" |
| Manifold | `eef0003` | yes — "mentored 5 relatively junior RBC engineers" |

`expert` is no longer an inference worth hedging.

**Daugherty `b0003` was linked in an earlier draft and removed on Jason's call:**
the mentoring there was informal advice on a mostly-senior team, and "coding
standards set within the pod" is standards-setting rather than mentorship. The
seven remaining links carry the skill without it. Recorded in the SQL so a later
audit does not re-add it.

**The caveat, now closed.** Only `ead0003` genuinely lacked leadership language;
section 6 adds "leading a team of 3-4 engineers". `ead0009` already read "Led a
team of approximately 5 engineers in Orlando and the UK" — an earlier draft of
this report said otherwise, which was wrong, read off a truncated excerpt.
Section 6 refines that into the two groups Jason distinguishes (architecture, 3;
Scrum, 5) rather than adding something absent.

Both corrections carry the **complete** replacement text rather than
concatenating onto the existing value, since these files are re-run and
appending would double the sentence.

Two things were deliberately *not* invented:

- The F-35 work maps to IOS (`ead0008`). The separate JSF support-center row
  `ead0010` is `is_active = false`, deliberately omitted as process-heavy; an
  inactive contribution is not linked.
- Manifold is described as "me plus about 6 junior devs in the office for the
  entire time." `eef0003` records mentoring 5 **RBC** engineers — the customer's
  people. The in-office cohort is a second, distinct population that no
  contribution records at all.

That last one is still the JFrog shape: real work, no row anywhere.

### Named in the doc, but explicitly *not* claims — deliberately excluded

Recorded so nobody re-extracts them later as missing skills. Extracting these
into the urgent list would have been actively wrong.

- **Hard excludes** (things to avoid): Ruby/Rails, LangGraph, CrewAI, RAG,
  crypto/blockchain, and Node.js/TypeScript, C#/.NET, Python *as primary stack*.
- **Recurring honest gaps** (things stated as lacking): Redis, Terraform, gRPC,
  payment rails/ACH, Kubernetes deep operational ownership.
  - Consistency check passed: **Redis and Kubernetes are seeded at `novice`** —
    which agrees with the doc calling them honest gaps. Terraform and gRPC are
    correctly absent as active skills.
- **Formatting Standards / Pipeline Steps sections**: `Node.js`, `docx`, `Arial`
  here describe the *legacy external generation script*, not career skills. The
  Node.js claim above rests on the Pelotech line, not on this section.

---

# 2. Seeded but not documented — lower priority, informational

**38 of 62** active skills are not mentioned in `canonical-context.md`.

**This is very likely legitimate and needs no action.** The doc's own closing
line says so: *"Source of truth is the structured career record (Role Model DB),
not this file."* The doc is a per-thread summary, deliberately short — it was
never meant to enumerate the full skill set. A 38-entry shortfall is the
expected shape of a summary, not evidence of stale data.

| category | not mentioned in doc |
|---|---|
| AI & LLM | LLM pipelines, Prompt engineering |
| Cloud & Infrastructure | AWS, EC2, ECS, S3 |
| Databases | DynamoDB, MySQL |
| Frameworks & Libraries | Cobra, FastAPI, Hibernate, JPA, Pydantic, React, SQLAlchemy, Spring Boot |
| Languages | Ada, Lua, Swift |
| Methodologies | Agile, Concurrency, SOA |
| Observability | Dynatrace, Splunk |
| Protocols & Messaging | DDS, HLA, JSON Schema, REST |
| Testing | JUnit, Mockito, TDD |
| Tools & CI/CD | Git, Gradle, Harness, Jira, Maven, SonarQube, Vite |

Two things worth a glance, neither urgent:

- **Splunk and Dynatrace** are absent from the doc, yet `022_skills_added_outside_seed.sql`
  says they "back the observability bullets directly," and *observability* is a
  target domain in the doc's Job Search Targeting. The doc names the domain but
  not the evidence for it. Adding them to Canonical Facts would make the
  observability targeting self-supporting.
- **REST** is seeded at `expert` with eight aliases and is not in the doc at all.

Nothing here suggests stale or test data. All 38 trace to seed files in
`database/seed/`.

---

# 3. What was not done

- No database writes, no migrations, no writes to `database/seed/`.
- No draft statement for any entry lacking a clear evidence source in the doc
  (macOS in particular names no employer).
- No LLM call was used to expand or infer the documented list — extraction is
  literal, from the text named in each row's source column above.
