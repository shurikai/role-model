# Role Model

**Repository:** https://github.com/shurikai/role-model

A self-hostable, single-user service for AI-powered resume generation.
Stores detailed career history as structured data and generates tailored,
versioned resumes per job application using an LLM.

Designed with a clear path to multi-tenant deployment without requiring
a schema redesign.

## Status
Active development. Backend: employer/position/contribution CRUD, application
CRUD, tags and tag-categories CRUD, education/credentials/projects, JWT auth
(signup/login/me), JD signal extraction and resume generation pipeline,
Stage 0 import (extract + enrich + approve/reject), fit gate scoring with
skills and preferences tables, CORS. Frontend: auth shell plus the
application generation flow (JD paste → fit → generate → render), built on
Vite + React + TypeScript.

Resume rendering is built: a Python docx-renderer service (FastAPI +
python-docx) owns document output, and internal/renderer is the Go HTTP
client that calls it. Note that internal/renderer was deleted once as dead
code and later reintroduced with an actual consumer — the current package is
the client, not the old renderer implementation.

## Career neutrality

This system is being made usable by someone whose career is nothing like the one
it was built around. The data model was always close to neutral — employers,
positions, contributions, tags, skills, and preferences describe any working life.
The presupposition lived in the vocabulary wrapped around it.

**Rules that hold now:**

- **No prompt names a user.** `fit_narrative.txt` addressed "Jason" by name and
  assumed his pronouns; it is second person throughout. A prompt that needs to
  refer to the person says "you".
- **The persona is field-neutral.** `resume_body.tmpl` and `resume_summary.tmpl`
  opened with "expert technical resume writer specializing in senior and
  staff-level software engineering roles", and everything downstream pattern-
  matched to software regardless of input. Do not reintroduce a field into the
  persona line. The candidate's field is whatever the provided data says it is.
- **`required_skills` / `preferred_skills` are gated on NAMEABLE and CHECKABLE,
  not on being technical.** The old rule — "only technical skills and tools" —
  meant a posting whose requirements were clinical, interpersonal, or
  pedagogical extracted three empty arrays, which silently disables the 2a
  relevance apparatus *and* makes `ScoreCapabilityFit` return `Scored: false`.
  "Communication" is still excluded, because it is unfalsifiable; "ACLS",
  "ServSafe", "Epic", and "Spanish/English bilingual" are requirements.
- **A competency is a capability, not an *engineering* capability.** Same reason.
- **Worked examples in a prompt must not all come from one field.** The examples
  are what the model calibrates on. Where a rule is field-agnostic, show it in
  two fields — that is why the specificity rule now carries both a p99-latency
  bullet and a triage-time bullet, and why `framingStaff` no longer uses a
  cruise-API bullet from the private seed set.

- **The seniority ladder and the depth scale are user-owned rows, not enums.**
  `career_levels` and `proficiency_levels` (migration 020) replaced six
  hand-synced copies of the ladder — a SQL CHECK, a JSON Schema enum, a Pydantic
  `Literal`, the extraction prompt's list, and two Go switches — which had
  already drifted: extraction never emitted `manager`, `director`, or `vp`, so
  three DB-valid values were unreachable from a job description. See
  **Level vocabularies** below for the rules that hold now.

- **No signal field is a closed classification any more.** Migration 021 deleted
  `jd_signals.domain` and `jd_signals.work_type` rather than renaming them. Each
  had a free-text field in `screening_summary` answering the same question
  without the truncation, and the enum was losing to it in every stored row:
  four postings in four unrelated industries all extracted as `saas` while their
  `industry` strings read "B2B business intelligence", "sales data", "web
  marketing platform" and "AI-powered marketing cloud"; `work_type: "remote"`
  stood in for "fully remote with occasional office visits or offsites as agreed
  with manager". Read `screening_summary.industry` and
  `screening_summary.work_arrangement`. **Do not reintroduce a classification
  enum beside a free-text field that already answers the question.**
- **The vocabulary is named for what it does, not for one industry's word for
  it.** `primary_stack` → `core_practice`, `anti_pattern` → `dealbreaker`,
  `preference_type 'work_type'` → `'role_shape'`, `technical_*` →
  `capability_*`, `projects.repo_url` → `source_url`, and `projects.role` /
  `projects.status` lost their CHECKs. The distinctions these carry are not
  software-specific — prominence versus presence is the same question for
  "Spanish fluency as a core requirement" as for "Python as a primary language".

- **The resume's shape is a manifest, not a call sequence.** `resume_sections`
  (migration 022) holds which content blocks print, in what order, under what
  heading. It replaced a shape fixed in three files at once — twelve required
  keys in the document schema, five renderer calls in a fixed order in
  `docx_builder.py`, and the heading strings written into each renderer's body
  — so renaming EDUCATION or moving CREDENTIALS above EXPERIENCE was a change
  to Go, Python, and a JSON schema. See **The section manifest** below.

- **The matcher no longer assumes technology-name morphology.** The direct
  layer is whole-word like the other two (#75), `tokenize` folds regular
  plurals, `core_competencies` is scored as a third input (#72), and preference
  labels carry an `aliases` column (migration 023) — which turned out to be the
  one mechanism three of the four fit-gate known gaps were missing.

**The import is reachable now, by two paths that stay separate.** The narrow
one (`ImportHandler`, `contribution_drafts`) stages contributions against
employers and positions that already exist. The wide one (`IntakeHandler`,
`entity_drafts`) stages the employers and positions too, which is what a new
account needs — `ApproveDraft` requiring a `position_id` that already existed
is what made the import unusable by anyone who had not already typed their
career in by hand. Both have a frontend: `/import/...` for the narrow path,
`/import/career/...` for the wide one.

Do not unify them. They stage different things, and the narrow path is still
the right shape once an account has a career in it — a contribution against a
position that has existed for years does not need a resolver.

**Dependency edges are crossed explicitly in both directions.** Approving a
draft whose parent is still pending is a 409 naming that parent, never a
silent cascade that writes rows the reviewer never saw; rejecting a draft with
dependents warns about what it strands rather than deleting them. Neither
direction resolves on the reviewer's behalf — that is the whole function of a
review queue.

**Still not neutral, and tracked in the plan** (`~/.claude/plans/i-want-to-do-bright-wall.md`):
Phase 9 is proving the whole thing with a second career, end to end through the
UI rather than through `cmd/intakerun`. This section covers Phases 1-6.

### The section manifest

`resume_sections (user_id, key, heading, sort_order, hidden, source)`. `key`
names a content block the pipeline produces; `heading` is the printed text and
the half the user owns. Four rules hold it together:

- **`key` and `source` mean different things, and `source` keeps the meaning it
  has on the vocabulary tables** — provenance (`default | inferred | user`).
  The plan proposed `source` for "which content feeds this section"; that would
  have put two meanings under one column name in sibling tables, which is
  precisely what `work_type` cost (#53). The content pointer is `key`.
- **Hidden reaches the renderer as ABSENT, never as a flag.** Generation drops
  hidden rows from the document's `sections` array. A renderer given a flag has
  to be trusted to check it, and the one that forgot would print a heading the
  user had explicitly turned off; a renderer given nothing cannot.
- **An unknown `key` is skipped, not rejected.** Otherwise adding a section type
  becomes a lockstep deploy — a document naming PUBLICATIONS would fail against
  every renderer that had not shipped yet, rather than rendering everything
  else.
- **An empty manifest means "no manifest", not "print nothing".** Both sides
  fall back to the conventional order. Reading an empty array as "every section
  is off" renders a page with a name on it and nothing else — total data loss
  that still returns 200.

A section still needs a content block behind it: this makes the *shape* of a
resume data, not its content. The cheapest route to a genuinely new section type
is that `projects` is already a list of dated named things with a role, a
status, links, and bullets — which is what a musician's PERFORMANCES and an
academic's PUBLICATIONS both are.

### Level vocabularies

`career_levels` carries the ladder **and the two levers a rung drives** —
`length_budget` (how much gets written) and `framing_guidance` (at what
altitude) — as sibling columns on one row. That relationship is the same one
the two Go switches used to encode, and it is why a third lever belongs beside
them as a third column rather than as a third lookup somewhere else.

- **Nothing compares level strings any more; code compares `rank`.** A rung's
  own `value` outranks another rung's `aliases` entry, the same precedence the
  fit-gate matcher keeps between direct and alias matches — otherwise whoever
  wrote an alias silently decides where a level resolves.
- **The fallback is a flagged row (`is_fallback`), never the median rank.**
  Deriving it looks neutral and is not: on the ten-rung software ladder the
  median rung is staff, so every posting whose seniority read `unknown` would
  have inherited the ownership framing that `framing_guidance` spends two rules
  warning against reaching for. An unrecognised seniority is not evidence of a
  senior role. A partial unique index enforces at most one per user.
- **The CHECKs were dropped, not converted into foreign keys.** A composite FK
  onto `career_levels` would trade one closed vocabulary for another — a new
  account starts on the neutral three-band set, so the first position filed as
  `staff` would be rejected by the database. Both readers already degrade
  gracefully: the level lookup falls through to the fallback row, and
  `LevelScale.Rank` ranks an unknown value at 0, below every band, so a typo
  reads as "no level established" rather than as clearing an expert bar.
- **Three creators populate these tables, and none of them covers every
  account.** Migration 020 backfills accounts that already existed with the
  ladder the code used to hardcode, so their behaviour is byte-for-byte
  unchanged. `vocabulary.Install` seeds accounts created through signup, inside
  the same transaction as the user row. `database/sample/001_foundation.sql`
  seeds its own user, because neither of the other two reaches a user a seed
  file invents — that is the #74 failure shape exactly, and
  `TestSampleSeedCarriesLevelVocabularies` is the guard.
- **The shipped default set is neutral, not software's.** `entry` /
  `experienced` / `senior`, because a default lands in the user's own data,
  where it is harder to notice and harder to remove than a Go switch was.
  Seeding a chef's account with `staff` and `principal` is the failure this
  guards against; `TestShippedLadderShape` fails if those names return.
  `internal/vocabulary` is a starting row set, not a lookup table — nothing in
  the pipeline reads it at runtime except the safety net for an account with no
  rows at all.
- **The extraction prompt's seniority enum renders from the user's rows.** That
  is what closes the drift: the list a JD is scored against and the list the
  database accepts are now the same list.

## Stack
- **Language:** Go
- **Router:** chi
- **Database:** PostgreSQL via pgx/v5 (native interface, not database/sql)
- **Query generation:** sqlc
- **Migrations:** golang-migrate
- **LLM:** Anthropic API via official Go SDK
- **Prompt storage:** internal/generation/prompts directory, embedded via go:embed
- **Prompt templating:** text/template
- **JSON schema validation:** santhosh-tekuri/jsonschema
- **Renderer:** Python 3.14 (FastAPI + python-docx), managed with uv,
  run as a separate process — not part of the Go binary
- **Frontend:** React + TypeScript on Vite, TanStack Query, Vitest

## Architecture

### LLM pipeline
1. **JD signal extraction (Stage 1)** — takes raw job description text, returns
   structured jd_signals JSON: required/preferred skills, core competencies,
   core practice, skill levels, seniority, culture signals, and a screening
   summary carrying the posting's own words for its industry and working
   arrangement.

   **A JD's requirements arrive in two shapes, and both are extracted.**
   `required_skills`/`preferred_skills` hold named, checkable requirements.
   `core_competencies` holds the capability-level asks a posting states in prose — "decomposing a
   legacy service", "production ownership of services", "setting technical
   direction". Senior and staff postings routinely name no technology at all,
   which left both skill lists correctly but uselessly empty and degraded every
   consumer at once and silently: the 2a requirement checklist rendered
   "(none listed)" twice, disabling the prompt's entire skill-relevance
   apparatus so it fell back to emitting the whole tag inventory, while
   `ScoreCapabilityFit` reported a vacuous 100 against nothing.

   The two lists stay separate because they are **satisfied** differently. A
   required skill can be answered by a Skills entry; a competency can only be
   evidenced by a bullet. Never let a competency into the Skills list — a
   resume listing "setting technical direction" among its technologies reads
   as padding and displaces a real skill.

   A third shape sits alongside them: `core_practice`, what the posting frames
   the role as being **practised in**. It is a strict subset of
   `required_skills` and answers a different question — not "does the JD ask for
   this" but "is this what the role is made of" — and it exists because the fit
   gate had preference rows making claims about prominence with nothing to check
   them against. Interchangeable alternatives keep the `" | "` grouping, and
   there the grouping is load-bearing rather than a convenience.

   Neither `core_competencies` nor `core_practice` is in the document
   projection. Per the intermediate-JSON rule below, adding a field to
   `JDSignals` must not change what the document emits.
2. **Resume generation (Stage 2)** — split into two calls:
   - **2a body** (`resume_body.tmpl`) — selects and writes bullets and
     skills against jd_signals, under a seniority-informed length budget
     and framing guidance

     **Seniority drives two levers, and they are siblings on purpose.**
     `length_budget` sets how much gets written; `framing_guidance` sets what
     altitude it is written at. Length was for a long time the only one, so a
     staff posting got more bullets of the same altitude rather than bullets
     pitched at the level it was hiring for — every other rule in the prompt
     pushes toward implementation specificity. Add a third lever to that pair,
     not somewhere else.

     Both are columns on one `career_levels` row now, resolved by
     `seniorityLevers` / `pickCareerLevel` in generate.go. They were two Go
     switches over a hardcoded ladder, and the pair had already drifted apart:
     `mid` took the short budget in one and the default branch in the other.
     See **Level vocabularies** above.

     Top-of-ladder framing adds ownership and scope **on top of** the evidence,
     never in place of it. Trading the metric for the claim is the failure
     mode, not the goal: the number is what makes the ownership claim
     believable, and a broad claim with nothing behind it is what a skeptical
     reader discounts.
   - **2b summary** (`resume_summary.tmpl`) — writes the summary scoped to
     the bullets 2a already selected, so it cannot assert unsupported claims

   Facts that both calls would otherwise decide independently (e.g. the header
   title) are threaded through as explicit inputs rather than re-derived. This
   is the established pattern for cross-call consistency — follow it.

Between the two calls, `reconcileSkills` enforces the bullet/skills invariant
2a states but does not reliably honour: a claimed skill that an emitted bullet
names must appear in the Skills section. It runs before 2b so the summary is
written against the final Skills list. Three rules hold it together:

- **Re-add only, never drop.** A skill can be legitimately claimed without a
  dedicated bullet; dropping on that basis would delete a JD-relevant skill for
  want of room in the bullet budget.
- **Only claimed skills are eligible.** A bullet naming WAGO or NGTS must not
  manufacture a skill with no `skills` row behind it.
- **Whole-word matching, never substring** — the same rule the fit-gate matcher
  documents, for the same reason. Substring matching makes "Go" a hit inside
  "Golang" and "Java" a hit inside "JavaScript". The boundary test is
  "not alphanumeric" rather than a regexp `\b`, because real skill names carry
  punctuation (`C++`, `C#`, `.NET`, `CI/CD`) that `\b` breaks on.

Category order is preserved on rewrite. The renderer prints categories in
document order, so decoding to a plain map would silently re-alphabetize the
resume's Skills section as a side effect of adding one entry.

Both calls are recorded separately in generation_params for per-call
traceability.

### Fit gate
`internal/fitgate` runs a deterministic pass before generation, evaluating two
axes in Go. The LLM only writes prose narrative from the result — it interprets,
it does not score.

The two axes are orthogonal on purpose: technical fit measures *capability*,
preference fit measures *desire*. A role you could do and would hate should read
as high technical / poor preference, not as one muddled number. Do not introduce
a blended score.

**They are also shaped differently, and that is not an inconsistency to tidy
up.** `ScoreCapabilityFit` returns a score, because coverage of stated
requirements genuinely is a ratio — matched over asked-for. `ScorePreferenceFit`
returns **four lists and no number at all**: `matches` (positives the JD
answers), `gaps` (positives it is silent on), `conflicts` (matched negatives),
and `gateHits` (matched hard gates).

Preference fit used to return a 0-100 float — a normalized weighted average,
minus a raw-points penalty for gate hits, clamped under a `hardGateCeiling`.
Migration 016 dropped `fit_reports.preference_score` and deleted all of it.
Nearly every preference defect this project has fixed lived in that arithmetic
rather than in the matching: unmatched negatives paid out a bonus because an
absent dislike earned its weight, gate hits read as a ten-point dip until the
ceiling was bolted on, and a gates-only profile scored a perfect 100 through the
empty-average short-circuit. Each fix added another interacting term to a number
nobody could read back to the rows that produced it. **Do not compute a
preference number anywhere in the backend** — not in `scorer.go`, not in the
narrative prompt, not in the API shape. An at-a-glance summary is a frontend
concern.

**The four lists must stay four lists.** Unmet preferences (the JD simply
doesn't mention something) and genuine conflicts (the JD involves something
actively unwanted) collapse into false conflict language. `gateHits` is
separate from `conflicts` for the same class of reason: a disqualifier and an
ordinary matched negative are different *kinds* of finding, not different
weights of one, and folding them together makes an exclusion read as one more
dislike. A gate hit appears in `gateHits` only — it used to be copied into
`conflicts` as well, which was meaningful only while a single score was
collapsing them anyway.

An unmatched negative — gate or not — is reported nowhere. Avoiding a stated
dislike is the ideal outcome and there is nothing to say about the absence of
an absence.

The narrative prompt sees a projection of these rows (`narrativePreference`:
label and type), not `db.Preference`. Weight is deliberately withheld — handing
the model the numbers invites it straight back into ranking rows arithmetically.
Which list an entry arrived in is the whole of its severity.

**A technical score can be absent, and absent is not zero and not 100.**
`ScoreCapabilityFit` returns a `CapabilityFit` whose `Scored` field is false when
the JD stated no technical requirements at all. It used to return a bare 100
there — a perfect score with no matches and no evidence, which the narrative
then wrote confident coverage prose around. "This profile answers none of the
requirements" and "this JD stated no requirements to answer" are opposite
findings and must not share a representation; that is why it is a struct field
rather than a sentinel value. When `Scored` is false the report stores SQL NULL
(the UI already renders that as "—") and the narrative input omits the score
entirely, which the prompt reads as "nothing was assessed". An empty
`capability_gaps` in that state means nothing was checked, not that there are no
gaps.

**Preferences carry severity and gate behavior separately.** `sentiment` is
`positive|negative`, `weight` is NOT NULL, and `is_hard_gate` marks the rows
that disqualify. A hard exclude is a heavy negative that also gates — there is
no `hard_exclude` sentiment (migration 011 removed it).

`weight` is still NOT NULL and still carried on every row, but nothing in the
scorer reads it any more. It survives because it records how much the user
cares, which a future presentation layer will want and which no other column
holds. Do not read it back into the backend to build a ranking.

Gating does **not** block. A tripped JD is still evaluated, still narrated,
still generated; the trip is a named finding in `gateHits`, and
`dealbreakers_clear` is derived from that list rather than being the only
record of it.

**Technical matching runs in three layers, strongest first**, and the layer
that won is recorded on every match as `kind` (`direct` | `alias` | `category`):

1. **direct** — the JD term against the skill's own name. **Whole-word, like
   the other two.** It kept a raw-substring direction until #75, justified by
   one case — "SQL" ⊂ "PostgreSQL" — which it paid for three ways: it
   over-reached ("API" answered by Anthropic API and FastAPI, "systems" by
   Distributed systems, and on ordinary English "art" by "charting" and "care"
   by "Medicare"); it **shadowed more precise vocabulary**, because direct
   outranks alias and a term sitting inside another skill's name could never be
   given one; and measured against the whole eval corpus the only requirement
   it reached that whole-word matching did not was a JD term "Go" answered by
   the skill "ArgoCD". PostgreSQL and MySQL carry `sql` in `tags.aliases` now,
   so the case that justified it is answered by the alias layer — on the
   specific skill, rather than by a letter coincidence. **Do not reintroduce a
   substring direction; add an alias.**
2. **alias** — against `tags.aliases`. That column had been populated since
   migration 001 and read by nothing, so "Golang" scored a gap against a
   stored "Go" and "RESTful APIs" against "REST".
3. **category** — against the tag's category name and `tag_categories.aliases`
   (migration 012). This is what lets a competency-worded JD reach a
   technology-worded profile: "CI/CD" is answered by Jenkins and GitHub
   Actions, "observability" by Splunk and Dynatrace. A JD that names no
   concrete technology at all — common at staff level — otherwise scores 0
   with every requirement reported as a gap.

Two rules hold this together:

- **Everything is whole-word matched, and regular plurals fold.** A category
  alias is a sentence fragment, and substring-matching one made a JD requiring
  "RAG" match the Testing category — "rag" sits inside "test cove*rag*e" —
  offering JUnit as evidence of retrieval-augmented generation. `tokenize`
  folds a trailing "s" on tokens of four characters or more that do not end in
  "ss", so "APIs" answers "API" and "patient assessments" answers "patient
  assessment". It is applied to both sides, so it only ever widens: a pair that
  matched before folding still matches after. It is not a stemmer and must not
  become one — "Redis" folds to "redi" and that is fine, because both sides do
  and nothing else folds there. Irregulars ("criteria"/"criterion") are alias
  data, not string logic.
- **A category alias must name a capability, not a technology.** Putting
  "kafka" on Protocols & Messaging would grant the whole category for one tool.
  The converse also bites: bare "frameworks" is deliberately not an alias of
  Frameworks & Libraries, because "auth/authz frameworks" and "evaluation
  frameworks" would both claim credit for React.
- **The seed owns `tag_categories.aliases`, not a migration.** Migration 012
  introduced the column and seeded it with `UPDATE ... WHERE name = '...'`, but
  migrations run before `make seed` and the categories do not exist yet — the
  UPDATEs matched zero rows, nine of ten categories sat on NULL, and the whole
  category layer was inert in the live database while every unit test stayed
  green, because they all build `CategoryAliases` inline. A staff posting
  requiring "APIs" scored it as a gap against REST at expert (#74). **Whoever
  creates a row populates its columns** — a migration cannot seed rows the seed
  has not created yet. Both `001_foundation.sql` seeds carry the vocabulary in
  the same INSERT that creates the row, and the data statements have been
  stripped out of migrations 012, 015, 018 and 019 entirely: **migrations own
  structure, seeds own vocabulary.** `TestSampleSeedCarriesCategoryVocabulary`
  is the guard, and it reads the seed file because no Go-level fixture can see
  an empty production column.

**One term can map to many skills, and there are two mechanisms with different
behavior.** A **category alias** maps a term to a category, and evidence is
every skill in it — the set *auto-follows*, so a tag added to Observability
later joins that evidence automatically. A **shared tag alias** maps a term to
an explicit list: the alias layer does not stop at the first hit, so the same
string on several tags accumulates into one match carrying all of them. That is
the only way to express a capability whose evidence spans categories, because
`tags.category` is a single NOT NULL column and categories strictly partition
tags — "backend systems" needs Java (Languages), Microservices and Distributed
systems (Methodologies), and REST (Protocols & Messaging) at once.

Prefer the narrowest layer that answers the term. Migration 018 made a JD
requiring "APIs" match through the Protocols & Messaging category, with DDS,
DIS and HLA — distributed-simulation protocols — in the evidence; naming REST
directly (migration 019) returns REST alone, and because the layers run
direct → alias → category, the precise alias demotes the broad match on its own
without removing the fallback.

The cost of the shared-alias mechanism is recorded rather than hidden:
`tags.aliases` means "other names for this tag", and "backend systems" is not
another name for Java, so the column now carries two relations. The match
reports `Kind: alias`, overstating a competency roll-up as a synonym; the
mapping has no single home; and it does not follow new skills. A
`competency_terms` / `competency_term_tags` pair with its own `MatchKind` is the
correct model. Build it when the term list outgrows a handful or someone needs
to ask what a term currently covers — `TestSampleSeedCarriesCrossCategoryTerms`
stands in for the constraint the schema does not express.

**The seed tests are split structural from content.** The structural half
(`TestSeedCategoryAliasesAreUnambiguous`, `TestSeedTagAliasesDoNotShadowOtherTags`,
`TestSeedCarriesSomeCategoryVocabulary`) asserts what the matching mechanism
needs of *any* dataset and names no technology. The content half reads its
expectations from `database/sample/vocabulary.json`, owned by the dataset, so a
second career ships its own file rather than breaking the first one's — the
whole test file used to name nine software categories inline, which meant
swapping in a non-software sample failed `make test`.

The structural tests deliberately do **not** require every category to carry
vocabulary. `Languages`, `Frameworks & Libraries` and `Cloud & Infrastructure`
are on NULL on purpose: bare "languages", "frameworks" and "cloud" would each
grant a whole category to any posting using the word.

Every match carries `evidence` — the specific skills behind it — so the
narrative cites what the person actually has instead of asserting a score, and
so a remaining gap is trustworthy. Gaps previously conflated "named
differently" with "does not have it".

**A match can be partial, and partial is a third verdict — not a weak match and
not a soft gap.** `jd_signals.skill_levels` is a sparse side table recording the
depth a posting states for a specific requirement ("expert-level Kafka", "5+
years of Go"), on the same scale as `skills.proficiency` — the user's own
`proficiency_levels` rows, read through `LevelScale`. Where a
requirement carries one and the strongest proficiency behind its evidence falls
below it, the requirement earns half credit and is filed in
`CapabilityFit.Partial` / `fit_reports.capability_partial`.

Four rules hold this together:

- **No stated level means no change.** Most requirements carry no entry, and
  that is the correct case rather than an extraction gap. A JD whose signals
  hold no `skill_levels` scores byte-for-byte what it scored before the feature
  existed — `TestFitEval` fails any fixture that produces a partial without one.
- **`pointsPossible` never moves.** Discounting the denominator alongside the
  numerator would cancel the penalty and make a partial score like a full match.
- **Level is consulted only after presence.** A requirement nothing answers is a
  plain gap whether or not a depth was stated, and the gap keeps the JD's
  phrasing. "You do not have this" and "you have this but not deeply enough" are
  different findings.
- **Match kind and level are orthogonal axes.** `Kind` says how evidence was
  found; the level fields say whether it clears the bar. A match can be direct
  and partial at once. Do not collapse them into one enum.

`SkillMatch.RequiredLevel` / `EvidenceLevel` are empty when no level was
assessed — deliberately not a `LevelMet bool`, for the same reason
`CapabilityFit.Scored` is not a sentinel score. A false would read as "the bar
was missed" while meaning "no bar was set". Carrying both levels also makes the
comparison auditable rather than a verdict to trust, and `LevelSignal` quotes
the JD wording it was read from.

`" | "` groups are looked up whole. `jd_extraction.tmpl` emits a level for a
group only when the depth applies to every alternative — a posting reading "deep
expertise in Kafka, or familiarity with Kinesis" gets no entry, because the
group can be satisfied through the alternative that carries no bar. Losing the
signal is the safe direction; asserting a bar the posting never held anyone to
is not.

**One matcher.** `prefFieldsFor` routes every preference by `preference_type`;
there is no second matcher for the gate, and `dealbreaker` is the only branch
that reads `required_skills`. The previous split (a broad `signalFields` for
scoring, a routed `gateFieldsFor` for the gate) is what hid #49: scoring never
saw the skills arrays, so a technology-shaped negative could not fire and,
because an unmatched negative earns its weight, paid out a bonus instead.

**Prominence is a separate signal from presence, and the types encode which one
a row is about.** `jd_signals.core_practice` holds what the posting says the role
is *built on*; `required_skills` holds only that a technology is asked for. A
preference whose label claims prominence — "Python as a primary language",
"Angular as co-equal frontend requirement" — is typed `core_practice` and routed
at that field. A preference where presence *is* the objection — "C# / .NET",
"crypto / blockchain" — stays `dealbreaker` and keeps `required_skills`.

Routing the first kind at `required_skills` is what made #68: the qualifier had
nothing to be checked against, so it was inert text, and `matchesSignal`'s
field-inside-label direction matched the bare token instead — `"Python"` is a
whole word inside `"expert Python as primary requirement"`. Every posting naming
Python as a required skill tripped a hard gate, capped at `hardGateCeiling`, and
was narrated as gating on expert Python. Do not add a prominence-claiming label
under `dealbreaker`; the qualifier will read as documentation and behave as
nothing.

Two rules specific to `core_practice`:

- **Alternatives are never split there**, which is the one place it differs from
  `dealbreaker`. A `" | "` group means the posting offers substitutes, and a
  technology you can be excused from is not what the role is built on. Splitting
  `"Java | Python"` back apart is how a JD reading "Proficiency in Java and/or
  Python" tripped the Python gate. `dealbreaker` still splits, correctly: it
  asks whether the JD demands the technology at all.

  **And not splitting a group is not, on its own, protection.** `containsPhrase`
  matches a token run anywhere inside the field, so `["python"]` sits inside
  `"Java | Python"` whether or not the group was split. What actually held the
  rule up for a year was label length — every gated label happened to be prose
  too long to fit inside a two-token group — which is an accident, not a
  mechanism, and a one-word label or any alias defeats it (#94). The rule is
  enforced positively now, in `corePracticeAnswered`: **a `core_practice` entry
  is answered only when the row answers every alternative in it.** A row
  refusing both halves of `"TypeScript | Node.js"` is not being offered an out
  and still fires; a row refusing only Python is, and does not. The row is what
  must cover the group, not any single term in it — label and aliases can refuse
  different alternatives and together close it.

  This is what makes vocabulary safe on these rows. An alias here is a bare
  technology name by nature, which is exactly the shape that reaches inside a
  group, and it is why `preferences.aliases` was populated for 19 rows and
  deliberately withheld from every `core_practice` row until this rule existed.
- **`collapseSubsumed` runs first.** Postings state their stack twice — a choice
  in the must-haves, a flat list under "Core Technical Stack" — and an entry
  whose alternatives are a strict subset of another's is the looser restatement
  and is dropped. `jd_extraction.tmpl` asks for the same deduplication, but a
  prompt is a request; this makes extraction noise harmless rather than unlikely.

**A preference is matched against the fields it is *about*, and a closed field
could not carry most of them.** `signals.WorkType` was
`remote|hybrid|onsite|unknown` and `signals.Domain` was one enum value, so
labels routed at those alone were not merely unlikely to match — roughly 50 of
139 possible weight could not match any JD, ever (#53). Both enums are gone
(migration 021). `role_shape` reads `culture_signals` and `core_competencies`;
`culture` reads `screening_summary.work_arrangement` alongside them; `domain`
reads `screening_summary.industry` and `core_competencies`.

`industry` and `work_arrangement` are the only screening fields any preference
reads. They answer what the enums answered, without the truncation — a
freight-visibility platform used to extract as `saas` while the industry said
"freight logistics visibility platform", and reading the enum reported
"logistics", the strongest positive in the profile, as an unmet gap on the JD it
matched best. Location, clearance, and `other_flags` are still read by nothing;
that is #48.

A consequence worth knowing before adding rows: **two labels that tokenize to the
same bag of words will both match the same JD.** `matchesSignal` compares token
runs and cannot tell "X over Y" from "Y over X", so the seeded pair "product over
platform / internal tooling" (positive) and "platform / internal tooling over
product" (negative) earned and conflicted at once. Migration 015 split them into
disjoint labels; the ordering the wording tried to express is already carried by
the weights. Write labels that share no vocabulary with their opposite.

Conditional preferences are modelled by **decomposing the root cause into its
own weighted row**, not by a dependency edge. "C# is only bad because of the
Microsoft ecosystem" is two rows — a gate on the ecosystem, a moderate
negative on the language — and additive weights produce the conditional
behavior on their own. Do not add a parent/implies relation to `preferences`.

### Intermediate resume JSON
Generation produces a structured JSON document (see /schema/resume.v2.json)
that is the contract between the generation pipeline and the renderer.
The renderer never touches the database. The JSON document is self-contained.

**Nothing is copied into the document verbatim from an evolving upstream
type.** `meta.jd_signals` is a deliberate projection — `documentJDSignals` in
internal/generation, mirroring `$defs.jd_signals` field for field — not the
stored `jd_signals` blob. The schema sets `additionalProperties: false`, so
assigning the blob straight through coupled a strict contract to a type owned
by extraction, and it broke in both directions at once: 15 stored applications
carried the deprecated `priority_skills`/`domain_vocabulary` and 5 carried
`screening_summary`, none of them declared. 20 of 31 applications could not
generate at all, each failing validation on a field the document never needed.

The rule that follows: **adding a field to `JDSignals` must not change what the
document emits.** If the document should carry something new, the schema
declares it first and the projection follows — never the reverse. The same
reasoning applies to any future blob the document embeds.

The `screening_summary` **block** is deliberately absent from the document. It
is screening data (location, travel, clearance, comp), not resume content; the
renderer has no use for it, and it is already persisted on
`fit_reports.screening_summary`.

Two of its fields do reach the document, by name: `industry` and
`work_arrangement`, which replaced the `domain` and `work_type` enums one for
one when migration 021 deleted them. The document carries the same two facts it
always did, untruncated. That is a projection choice, not the block leaking —
`TestForDocumentDropsScreeningSummary` asserts both halves, that those two
arrive and that location, travel, clearance, and `other_flags` do not.

### Renderer
Built. `docx-renderer/` is a small Python service (FastAPI + python-docx)
exposing a single `POST /render` endpoint that takes the intermediate resume
JSON and returns a `.docx`. The Go service reaches it through
`internal/renderer.Client`, surfaced as
`POST /api/v1/resume-versions/{id}/render`.

The renderer never touches the database — the JSON document it receives is
self-contained. Layout is explicit and compact: it does not use Word heading
styles, and it sets `keep_with_next` on section-heading and role-header
paragraph chains for widow/orphan protection. Bullets are deliberately left
free to break across pages.

`build_resume_document` iterates the document's `sections` manifest and looks
each `key` up in `SECTION_RENDERERS`. Identity is not a section — it is the
document header, always first, never in the manifest, because a resume with its
name turned off is not a configuration anyone wants. Each section renderer
keeps its own empty guard and returns before emitting a heading, which is the
invariant that stopped bare headings printing over nothing.

### Prompt management
Prompts live in /internal/generation/prompts, embedded into the binary at
compile time via go:embed.

**Prompt filenames carry no version number.** A prompt's identity is the git
blob hash of its content, computed from the embedded bytes by
`promptFingerprint` and recorded in `resume_versions.generation_params`. This
is the same hash git computes for the same bytes, so a recorded blob resolves
directly against the repository:

```
git cat-file -p <blob>          # the exact prompt text used
git log --find-object=<blob>    # the commit that introduced it, and why
git diff <blobA> <blobB>        # what changed between two generations
```

A prompt's *history* is `git log` on its path; its *rationale* is the commit
message plus the `{{/* */}}` header in the file.

Rules:
- **Never put a version number in a prompt filename or in a Go constant.** The
  scheme exists because a filename and a constant are two sources of truth for
  one fact, and they drifted — resumes were recorded against a prompt file that
  did not exist. Do not reintroduce that.
- **Commit prompt changes before generating anything you need to trace.** An
  uncommitted edit still hashes correctly and stably, but the blob exists in no
  commit and cannot be recovered later. `make check-prompts` warns; it runs
  automatically before `make run` and `make dev`. It warns rather than blocks
  on purpose — edit-and-regenerate is the normal tuning loop.
- Template `{{/* */}}` headers must end with the `-}}` trim marker, or the
  rendered prompt gains a leading newline. `TestPromptCommentsDoNotLeak` guards
  both the leak and the newline.
- `pipelineVersion` in prompts.go is separate and still hand-maintained. It
  names the *call sequence* (currently the 2a/2b split), which no individual
  file's content captures. Bump it when the shape of the pipeline changes, not
  when prompt text changes.
- `schema/resume.v2.json` requires a `prompt_version` field in the document
  `meta` block and sets `additionalProperties: false`, so the portable document
  carries `pipelineVersion` only. Per-prompt hashes live in generation_params
  on the DB row. Putting them in the document would require a schema v2 and a
  matching change to the renderer's Pydantic models.

## Project Structure
/cmd/server                      — main entry point
/cmd/resetpw                     — CLI to reset a user's password (no UI flow yet)
/internal/api/handlers           — HTTP handlers
/internal/auth                   — JWT issuance/validation, bcrypt
/internal/config                 — environment-based config loading
/internal/db                     — sqlc generated code
/internal/fitgate                — deterministic fit scoring + narrative
/internal/generation             — LLM pipeline (signal extraction + resume generation)
/internal/generation/prompts     — LLM prompt template files (embedded at compile time)
/internal/httputil               — shared HTTP helpers (breaks handlers↔middleware cycle)
/internal/intake                 — whole-career import: extraction, review flags,
                                   and the dependency-ordered resolver that turns
                                   approved drafts into rows
/internal/renderer               — HTTP client for the docx-renderer service
/internal/stage0                 — LLM-assisted import (extract + enrich + review)
/internal/vocabulary             — starting level vocabularies and section manifest,
                                   installed at signup
/docx-renderer                   — Python service: resume JSON -> .docx
/frontend                        — React + TypeScript + Vite UI
/database/seed                   — real career seed SQL; a separate private git
                                   repo checked out in place, gitignored here
/database/sample                 — fictional sample dataset, tracked here;
                                   loaded by `make seed-sample`, never `make seed`
/migrations                      — golang-migrate SQL migration files
/schema                          — JSON schema documents
/tests/fixtures                  — JD, resume JSON, and .docx regression fixtures
/notes                           — working notes

## Key Files
- /CLAUDE.md                 — project instructions and conventions (this file)
- /schema/resume.v2.json     — intermediate resume JSON schema (source of truth)
- /migrations/               — database migrations (source of truth for schema)
- /internal/generation/prompts/ — LLM prompt templates

## Data Model Decisions
- UUIDs for all primary keys, client-generated
- created_at / updated_at on all tables
- Soft deletes on contributions (is_active) and anywhere historical data has value
- user_id on all tenant-scoped tables (employers, positions, contributions,
  applications, resume_versions, etc.)
- JSONB for flexible blobs: jd_signals, generation_params, structured_output,
  edited_deltas
- Tags are user-defined with user-defined categories, normalized via aliases
- Positions carry both verbatim company title and industry-normalized level/role
  with a level_rationale field. **The rendered resume shows `industry_role`;
  `title` is the fallback when it is absent.** A verbatim title carries the
  employer's internal grade ladder rather than the job — "Programmer VII" tells
  a reader nothing about the work and reads as junior outside that company. The
  normalization was in the database from the start, survived every stage of the
  pipeline, and was discarded on the renderer's last line. `title` stays in the
  document for provenance; keep it there. `industry_role` is free text, not an
  enum, because real roles are compound ("Senior Software Engineer / Architect")
  — `industry_level` is the enum.
- **A contribution belongs to a position, a project, or both** (migration 014).
  `position_id` is nullable: it says *where* the work was done, and the
  `project_contributions` link says *what it was part of*. Those are independent
  facts, which is why a job contribution on public code carries both. Before
  this, personal projects had nothing to hang a contribution on, so
  `project_contributions` stayed empty and `assembleProjects` dropped every
  project before the prompt saw it — no generated resume could mention a
  project at all.

  What a contribution may not be is homeless: a row with neither is invisible
  to both assemblers and nothing would report it as unreachable. A deferred
  constraint trigger enforces this (a CHECK cannot subquery, and the invariant
  spans two tables), mirrored onto `project_contributions` DELETE — an
  invariant that holds only on insert is not an invariant.
- Contributions are richer than resume bullets: full_description, outcomes, and
  scale_context are separate fields to give the LLM distinct signals to draw from
- Bullet traceability: each generated bullet in the JSON carries contribution_ids
  linking back to source contributions
- Feedback is scoped per resume version, not per contribution globally

## GitHub Issues

This project uses `gh` (GitHub CLI) for issue tracking. The `gh` agent skill is
installed (`gh skill install cli/cli gh --scope user`) and should be used for any
issue or PR interaction rather than constructing raw `gh api` calls from scratch.

### Session workflow

**At session start:**
- If no specific task was given, run `gh issue list --label stage-2` (or the
  relevant label) to find the next queued item rather than assuming there is
  nothing to do.
- When picking up an issue, apply the `in-progress` label:
  `gh issue edit N --add-label in-progress`
- Leave a comment on the issue noting the session date and starting point:
  `gh issue comment N --body "Picking up in session YYYY-MM-DD. Starting from X."`

**During a session:**
- Reference issues in commit messages: `Refs #N` for related work,
  `Closes #N` only when the fix is verified working (tests pass, manually
  confirmed where relevant) — not just written.
- Do not close an issue automatically as part of a larger task unless explicitly
  asked. Surface "this looks like it resolves #N" and let the human confirm.

**At session end:**
- Leave a closing comment summarizing what was done, what was deferred, and
  any blockers: `gh issue comment N --body "..."`
- Remove the `in-progress` label when closing or suspending work:
  `gh issue edit N --remove-label in-progress`
- Close only on explicit human confirmation: `gh issue close N`

### Labels in use
- `stage-2` — resume generation pipeline work
- `renderer` — DOCX/PDF rendering
- `infra` — tooling, migrations, dev environment
- `backlog` — deferred, not forgotten
- `in-progress` — actively being worked in the current or most recent session

Apply an existing label rather than inventing a new one. Ask if a new label
seems genuinely warranted.

### What belongs in Issues vs. here
- Issues are the source of truth for *what's planned and tracked*.
- This file is the source of truth for *how to build it* (stack, conventions,
  the Do Not list below).
- Do not duplicate task lists here that belong in Issues.

## API Design
- REST
- JSON request/response
- Structured error responses, not raw strings
- Environment-based config, no hardcoded values
- JWT-based auth (24h token, no refresh — see internal/auth), single-tenant
  today but every table already carries user_id for a clean path to
  multi-tenant later

## Formatting
Each language has one pinned formatter, and `make fmt` runs all three
(`make fmt-check` verifies without writing):

- **Go** — `gofmt`, from the toolchain
- **TypeScript** — Prettier, pinned in `frontend/package.json` with
  `.prettierrc`; also `npm run format` / `format:check`
- **Python** — `ruff format`, pinned in `docx-renderer`'s dev group with
  `[tool.ruff]` in pyproject.toml

Prettier is not always idempotent — a first `--write` pass can emit output that
a second pass reformats (it happened on a `vi.fn().mockResolvedValue()` chain).
Run it to convergence; do not assume one `--write` satisfies a later `--check`.

**SQL is deliberately not formatted.** Migrations are applied history that must
not churn, and the sqlc query files carry load-bearing `-- name: Foo :one`
directives that comment-reflowing formatters can silently break. Do not add
pg_format or sqlfluff.

## Conventions
- No ORM — use sqlc generated code against pgx native interface
- No database/sql — pgx native only
- No framework beyond chi — stdlib patterns otherwise
- Errors returned as structured JSON: { "error": "message", "code": "slug" }
- All handlers receive a context, all DB calls respect context cancellation
- Config via environment variables, loaded at startup into a typed Config struct

## Dependencies

**The conservative dependency rule is about the Go service, and only the Go
service.** There, a dependency is a real cost — it ships inside a
single-binary server, it is a supply-chain surface for something self-hosted,
and the stdlib usually already answers the question. Adding one to `go.mod`
needs a reason a reviewer would agree with. `chi`, `pgx`, `sqlc` and the
Anthropic SDK are the shape of the exception, not the start of a trend.

**The frontend and the renderer follow their own ecosystems' normal practice.**
`frontend/` is a Vite bundle: dependencies are tree-shaken, versioned in a
lockfile, and ship only what is imported. A well-maintained library that
answers the need is the right call there — reaching for React, TanStack Query,
Tailwind, or an icon set is ordinary work and needs no justification beyond
"this is what it is for". The same holds for `docx-renderer/`, which is a
separate Python process with its own `pyproject.toml`.

This is written down because the rule was once read the other way round.
Session 034 hand-rolled nine inline SVG icons to avoid adding `lucide-react`,
citing the Do Not list below — a page of hand-maintained path data standing in
for a tree-shaken import that cost under 1 KB gzipped once measured. The
backend rule had been applied to a frontend decision. **Do not infer the Go
rule onto `frontend/` or `docx-renderer/`.**

What still applies everywhere: prefer a dependency that is maintained, prefer
one over three, and say in the commit message what it replaced.

## Do Not

The technical bullets here are about the **Go service** — that is where the
architectural commitments live, and most of them have no frontend or renderer
equivalent. See **Dependencies** above before reading one as project-wide. The
last two are workflow rules and apply to the whole repository.

- Use an ORM
- Use database/sql directly
- Use gin, echo, or any heavy framework
- Hardcode any user identity, file paths, or config values
- Add a Go dependency without a clear justification — this one is Go-only, and
  does NOT govern `frontend/` or `docx-renderer/`; see **Dependencies** above
- Store rendered document files in the database (blob storage interface goes here)
- Put business logic in HTTP handlers
- Invent prompt improvements — prompts live in /internal/generation/prompts
- Add a version number to a prompt filename or a prompt version constant — see
  Prompt management above; content hashing replaced both
- Open new issues unprompted during a session focused on something else. If you
  notice unrelated work that should be tracked, mention it and let the human decide.
- Use the `claude-code-action` GitHub App or any webhook-triggered automation.
  All `gh` usage is interactive, inside a human-initiated session only.

## Open Questions
- Blob storage interface for rendered documents (local disk now, S3 later).
  Rendered .docx bytes are currently returned to the caller, not persisted.
- Evaluation strategy for prompt quality across versions (deferred)
- Skill depth signal — **the gap is in the code, not the data.** The schema
  supports depth (`skills.proficiency`, `skills.years_experience`, and
  `v_skill_provenance` deriving skill→contribution links from
  `contribution_tags`), and the data now carries it: migration 008's uniform
  `proficient` / `NULL` backfill was curated afterward in seed files 016/018/019,
  so the table holds a real spread of novice/proficient/expert with
  `years_experience` populated on most rows.

  **Partly resolved as of migration 017.** `ListActiveSkillMatchTermsByUser`
  now selects `s.proficiency`, and `ScoreCapabilityFit` compares it against
  `jd_signals.skill_levels` — see the partial-match rules in the Fit gate
  section. What that closes is the *JD-side* half: a posting that asks for
  expert Kafka no longer scores a novice Kafka as a clean match.

  **The profile-side half is still open, and is the harder call.** Where a
  posting states no depth — most postings — a one-off prototype and a decade of
  production use still score identically, because nothing asked. Discounting a
  match for shallowness the JD never enquired about would rescore every
  application already in the database, which is a design decision rather than a
  bug fix. `known-gap-depth-blind-scoring` holds that case and stays a known gap.

  `years_experience` remains dropped at the query layer, deliberately: nothing
  compares against a number. Where a JD gives a years figure, extraction reads
  it as a level signal instead.

  **Generation reads depth directly; the fit gate reads only proficiency.**
  `assembleSkills`
  (`internal/generation/assemble.go`) selects the claimed skills with
  proficiency and years via `ListActiveSkillProfileByUser` and passes them to
  2a as `<skills>`, which filters on relevance and depth together and may
  annotate a few deep, central skills as "Java (25 yrs)". That block is also
  now the **only** source for the resume's Skills section — it used to be
  built from contribution tags, which are vocabulary rather than claims, and
  that is how JavaScript reached a rendered resume without a `skills` row
  behind it.

  A category match still earns full credit, and the strongest proficiency in
  the category answers any stated bar — one expert in Observability carries the
  whole category over an expert requirement. That follows from category evidence
  being every skill in the category, and it is visible in the report because
  `Kind` records that the match was a category match in the first place.

Resolved: the renderer service question (Go-native vs Python) — Python won,
see Architecture above.
