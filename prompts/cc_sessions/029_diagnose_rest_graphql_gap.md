# CC Session: Diagnose REST APIs False Gap / Missing GraphQL Gap

## Context

Latest fit report for the Citi Principal Java Engineer JD (after sessions
025-028) shows:

```
Technical score: 82.35/100
Technical gaps:
- RESTful APIs
```

Two things are wrong with this:

1. **"RESTful APIs" as a gap is false.** REST APIs is a confirmed canonical
   strength (see `canonical-context.md` in the project — Java/Spring Boot/
   REST APIs is core, longstanding experience). This should not be gapping.
2. **GraphQL is missing entirely.** The JD (full text below) requires
   "RESTful APIs and GraphQL implementations." GraphQL is a genuine,
   unhedged gap — no canonical GraphQL experience exists. It should appear
   as a gap and currently does not appear anywhere in the report — not as a
   gap, not as a match.

This session is diagnose-first, fix-second — do not guess at the mechanism
before confirming it against actual data, the way session 027 disproved its
own initial hypothesis by testing it. Follow Task 1 fully before touching
any code.

Stack: Go, `internal/fitgate/scorer.go` (`ScoreTechnicalFit`, `matchesAny`),
`internal/generation/{extractor.go,prompts/jd_extraction.tmpl}`.

---

## Session Start (always)

```bash
git pull
gh issue list --state open
```

---

## Task 1: Diagnose — get real data before forming a theory

**1a. Re-run Stage 1 extraction against the actual Citi JD** and dump the
raw `required_skills`/`preferred_skills` arrays as JSON. JD text:

```
Citi is looking for a hands-on Principal Java Engineer at Vice President level to design, build, and evolve high-quality software products that solve real business challenges at global scale. Principal Java Engineer you will leverage your deep technical knowledge to drive the creation of high-quality software products. You will also be expected to mentor other engineers, share your technical expertise, and promote a culture of technical excellence within the team. The Principal Engineer will report to an Engineering Manager and will be a floating member of multiple engineering teams. There is an expectation to contribute to the codebase and deliver solutions against the sprint-level commitments. From a technical standpoint, the Principal Engineer has full-stack coding and implementation responsibilities and adheres to best practice principles including modern cloud-based software development, agile and scrum, code quality, and tool usage. The Principal Engineer works to understand and influence software architecture, while contributing to Citi's and GFT's technical user base.

Requirements
10 or more years of software engineering experience building and delivering production-grade applications using Java, with a clear command of data structures and object-oriented design principles.
Proven depth in cloud-native development and container orchestration, with hands-on use of tools such as Docker, Kubernetes, or OpenShift across multiple years of delivery.
Extensive experience designing and building microservices and service-oriented architectures, including RESTful APIs and GraphQL implementations.
Strong working knowledge of Java frameworks including Spring Boot, Quarkus, Micronaut, or Vert.x, applied across real-world software delivery.
Practical experience with software engineering best practices including unit testing, automation, design patterns, and peer code review as core parts of the development workflow.
Familiarity with CI/CD pipeline tooling such as Tekton, Harness, CircleCI, or Jenkins, in either on-premise or public cloud environments.
Multiple years of working within agile and iterative delivery frameworks, contributing actively to sprint execution and team-level Scrum practices.

Nice To Haves
Architecture experience designing horizontally scalable, highly available, and low-latency distributed systems in cloud environments such as AWS or OpenShift.
Exposure to event-driven architecture patterns and streaming technologies, including Kafka, Spark, or Flink.
Familiarity with Infrastructure as Code tooling such as Terraform or CloudFormation, and API management platforms.
Experience with observability, security, and monitoring tooling including Grafana, Prometheus, Splunk, ELK, or CloudWatch.
Experience mentoring and providing technical leadership to teams of five or more engineers, with exposure to front-end technologies such as React or Angular and database concepts across RDBMS and NoSQL.
```

Confirm: does `required_skills` contain "RESTful APIs" and "GraphQL" as two
separate entries, one combined entry, a `" | "`-joined pair (possible
session-028-convention misfire — "RESTful APIs and GraphQL" could plausibly
get misread as offering alternatives, which would be wrong; "and" here
means both are required, not either/or), or is GraphQL missing from
extraction output entirely? Get the literal JSON, don't infer from the
report.

**1b. Query the actual skill name(s) stored for REST-related and GraphQL-
related skills:**

```bash
psql $DATABASE_URL -c "SELECT id, name, is_active FROM skills WHERE name ILIKE '%rest%' OR name ILIKE '%graphql%' OR name ILIKE '%api%';"
```

**1c. With both pieces of real data, determine the actual mechanism.**
`matchesAny` (`internal/fitgate/scorer.go`) is one-directional and a plain
substring check: `strings.Contains(strings.ToLower(skillName),
strings.ToLower(tag))` — the JD's extracted term must appear as a substring
*within* the stored skill name, not the reverse, and no word-boundary
handling (unlike `matchesSignal`/`containsPhrase` elsewhere in this file).
If the stored skill is named e.g. `"REST APIs"` and the extracted JD term
is `"RESTful APIs"`, `"restful apis"` is not a substring of `"rest apis"` —
that would produce exactly this false gap. Confirm or refute this against
what Task 1a/1b actually returned before writing any fix. If the actual
mechanism is different (e.g. the OR-list misfire from session 028, or
GraphQL genuinely absent from extraction output), the fix in Task 2 needs
to target *that*, not this hypothesis.

Report findings before proceeding to Task 2, even though this is a single
session — state plainly what the raw extraction output was, what the
stored skill name is, and which mechanism actually explains both symptoms
(the false REST gap and the missing GraphQL gap may not share one cause —
don't assume they do).

## Task 2: Fix based on confirmed root cause

Likely candidates, to be selected based on Task 1's actual findings, not
assumed in advance:

- **If it's a naming/normalization mismatch** (most likely per the
  `matchesAny` analysis above): the cleanest fix is extending
  `jd_extraction.tmpl`'s existing canonicalization rule — it already says
  "Go" not "Golang", "Kubernetes" not "K8s" — to include REST normalization,
  e.g. "REST APIs" not "RESTful APIs" or "REST API." Add explicit guidance
  and an example. This is a two-line prompt change, not a code change, and
  keeps the fix consistent with how the template already handles this
  exact class of problem for other terms.
- **If `matchesAny`'s one-directional substring check is the deeper
  issue** (i.e. naming normalization alone won't cover every future case):
  consider whether `matchesAny` should use the same `containsPhrase`
  word-boundary/bidirectional logic that `matchesSignal` already uses,
  instead of raw `strings.Contains`. Weigh this against scope — this
  function is called far more often (once per skill per JD) than
  `matchesSignal`, and bidirectional matching has different failure modes
  for skills specifically (e.g. a stored skill "Go" would then match a JD
  term "Google Cloud" bidirectionally in ways that wouldn't happen with a
  one-directional check). Do not make this change unless Task 1 shows the
  normalization fix alone is insufcient — flag it as a considered-and-
  rejected option in the session summary if you decide against it.
- **If GraphQL is dropping out during extraction** (e.g. the `" | "`
  OR-group convention misapplied to an "and" list): this is a
  `jd_extraction.tmpl` prompt-clarity fix — reinforce that `" | "` is only
  for genuine either/or alternatives, and "X and Y" in a single requirement
  sentence means both are required and must be extracted as separate
  entries. Add this specific sentence pattern as a negative example if the
  template doesn't already have one.

Whichever applies, confirm the fix by re-running extraction + scoring
against the same Citi JD and checking:
- REST APIs (however canonically named) no longer appears as a gap.
- GraphQL appears as a gap (assuming Task 1b confirms no GraphQL skill
  exists in the skills table — if one unexpectedly does, treat that as a
  separate finding to report, not silently resolve).

## Task 3: Broader check (only if Task 1 confirms a normalization-class bug)

If the root cause is naming normalization, do a quick pass — not a full
audit — checking whether any other common acronym/expansion pairs in the
extraction template's existing skill set could have the same failure mode
(e.g. "CI/CD" vs. "Continuous Integration," "K8s" already handled, "IaC"
vs. "Infrastructure as Code"). List anything found in the session summary.
Do not fix speculative cases without evidence they've actually occurred in
a report — this task is reconnaissance, not a preemptive rewrite.

---

## Do NOT

- Do not write a fix before completing Task 1's diagnosis with real data.
- Do not touch `matchesSignal`, `containsPhrase`, or anything in the
  anti-pattern gate — this session is scoped to `ScoreTechnicalFit` /
  `matchesAny` / extraction only.
- Do not modify the `skills` table directly (renaming stored skill names)
  as a fix — normalize on the extraction side, not the user's canonical
  skill data, unless Task 1 clearly shows the stored name itself is wrong
  or inconsistent with `canonical-context.md`, in which case flag it for
  Jason rather than changing it directly.

---

## Verification Steps

```bash
go build ./...
go test ./internal/fitgate/... -v
go test ./internal/generation/... -v
```

Add a regression test in `scorer_test.go` reproducing the specific
REST/GraphQL case with the actual skill name and extracted term found in
Task 1 — not a synthetic approximation.

Session is complete when: Task 1's findings are stated plainly in the
summary, the Citi JD's technical gaps correctly show GraphQL and not REST
APIs, the new regression test passes, and `go build ./...` / `go test
./...` are clean.
