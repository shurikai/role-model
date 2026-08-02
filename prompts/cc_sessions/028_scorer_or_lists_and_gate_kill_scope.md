# CC Session: Fix OR-List Double-Counting and Preferred-Skills Gate False Positives

## Context

Two scoring bugs surfaced from a real JD (Citi Principal Java Engineer),
both in `internal/fitgate/scorer.go` and its upstream data source,
`internal/generation/prompts/jd_extraction.tmpl`.

**Bug A — OR-lists get flattened and double-counted.** The JD requirement
"Java frameworks including Spring Boot, Quarkus, Micronaut, or Vert.x" is a
single requirement satisfiable by any one of four frameworks. Extraction
currently flattens this into four separate `required_skills` entries.
`ScoreTechnicalFit` then scores each independently: having Spring Boot
satisfies one of the four, and the other three are reported as gaps and
also count against `pointsPossible` as if they were four unrelated
requirements. Same pattern with "Tekton, Harness, CircleCI, or Jenkins" —
Jenkins is a confirmed strength, but Tekton and CircleCI both show up as
gaps for something already satisfied. This drags the technical score down
incorrectly and produces a misleading gap list.

**Bug B — `preferred_skills` gets equal weight to `required_skills` in the
anti-pattern gate.** `gateFieldsFor`'s default (`anti_pattern`) branch
includes both `signals.RequiredSkills` and `signals.PreferredSkills` when
matching hard-exclude preferences. On the Citi JD, Angular appeared exactly
once, inside a "nice to have" bullet ("exposure to front-end technologies
such as React or Angular") — landing in `preferred_skills`, not
`required_skills`. That single low-confidence mention was enough to trip a
hard-exclude with the narrative describing Angular as a "co-equal frontend
requirement," which the JD does not remotely support. A hard-exclude should
fire on genuine requirements, not optional nice-to-haves.

Stack: Go, `internal/fitgate/scorer.go`,
`internal/generation/{extractor.go,prompts/jd_extraction.tmpl}`. No DB or
LLM calls in `fitgate` — keep it that way; this stays deterministic.

---

## Session Start (always)

```bash
git pull
gh issue list --state open
```

Before Task 2, check actual stored hard-exclude preference labels
(`psql $DATABASE_URL -c "SELECT id, label FROM preferences WHERE
preference_type = 'anti_pattern' AND sentiment = 'hard_exclude';"`) so the
regression test in Task 2 uses real label text, not an approximation.

---

## Task 1: Represent OR-groups without changing the `[]string` schema

Do NOT change `JDSignals.RequiredSkills`'s type from `[]string` — it's
consumed downstream by resume generation (`resume_body.tmpl`, per the
extraction template's own header comment: "Output feeds both the fit gate
and pass 2a") and by the frontend (`frontend/src/lib/types.ts`,
`ApplicationDetail.tsx`). Widening the blast radius to a structured
alternatives type is not warranted for this fix — grep both consumers
first to confirm nothing else assumes one-skill-per-string before
proceeding, but the intended fix is a within-string convention, not a type
change.

**Convention:** when a JD explicitly presents technologies as
interchangeable alternatives (patterns like "X, Y, or Z", "any of X/Y/Z",
"one of the following: X, Y, Z"), extraction should emit **one**
`required_skills` (or `preferred_skills`) entry joining the alternatives
with `" | "` — e.g. `"Spring Boot | Quarkus | Micronaut | Vert.x"` — rather
than four separate entries. Do not use `"/"` alone as the delimiter; it
collides with real skill names that legitimately contain a slash (`CI/CD`,
`TCP/IP`). `" | "` does not appear in any real-world skill name and is safe.

This is **only** for genuine interchangeable alternatives, not for lists of
distinct required competencies joined by "and" — "Docker, Kubernetes, and
Terraform" stays three separate entries; nothing there is a substitute for
anything else.

Update `internal/generation/prompts/jd_extraction.tmpl`:
- Add an explicit rule under "Rules:" describing the `" | "` convention,
  using both real Citi JD examples above as worked examples (Spring
  Boot/Quarkus/Micronaut/Vert.x and Tekton/Harness/CircleCI/Jenkins).
- Be explicit about the distinguishing test: are these things the JD is
  offering as substitutes for the same requirement, or are they separately
  required?

Update `ScoreTechnicalFit` in `internal/fitgate/scorer.go` to parse each
`required_skills`/`preferred_skills` entry for the `" | "` delimiter:
- Split on `" | "` to get the alternative set (a plain entry with no
  delimiter is a set of one).
- The requirement is satisfied (full points — 2 for required, 1 for
  preferred) if `matchesAny` succeeds for *any* alternative in the set.
- If no alternative matches, append the *original undivided entry* (e.g.
  `"Quarkus | Micronaut | Vert.x"`, or better, the full original including
  Spring Boot if it too were unmatched) to `gaps` as a single item — not
  one gap per alternative.
- `pointsPossible` continues to count each `required_skills`/
  `preferred_skills` *entry* once (this already falls out correctly once
  extraction stops over-splitting — no change needed to the
  `pointsPossible` calculation itself, just confirm it with a test).

Also update `gateFieldsFor`'s default branch, which flattens
`RequiredSkills`/`PreferredSkills` into a field list for gate matching — it
should split `" | "` groups into individual tokens before adding them to
the fields list, so a hard-exclude can still correctly match an alternative
buried inside a group (e.g. a `"Ruby"` hard-exclude should still fire
against `"Java | Ruby | Python"` if that ever occurs).

**Add test coverage** (`scorer_test.go`) reproducing the exact Citi
scenario:
- `required_skills: ["Java", "Spring Boot | Quarkus | Micronaut | Vert.x", "Tekton | Harness | CircleCI | Jenkins"]`,
  `skillNames` including "Java", "Spring Boot", "Jenkins" but not
  Quarkus/Micronaut/Vert.x/Tekton/Harness/CircleCI.
- Assert: zero gaps from the two OR-groups (both satisfied), score
  reflects 2 requirements fully earned out of 2 possible (plus Java), not
  2-out-of-6ish under the old flattened behavior.
- A separate case where *none* of an OR-group's alternatives match: assert
  exactly one gap entry for that group, not N.

## Task 2: Drop `preferred_skills` from the anti-pattern gate

In `gateFieldsFor` (`internal/fitgate/scorer.go`), remove
`signals.PreferredSkills` from the default (`anti_pattern`) branch's field
list. Only `signals.RequiredSkills` (post-Task-1 splitting) should be
checked for hard-exclude skills matches.

Update the function's doc comment to state explicitly that preferred/nice-
to-have mentions are excluded from gate matching by design — a hard-exclude
should reflect a genuine requirement, not an optional, low-confidence
mention. `preferred_skills` still feeds `ScoreTechnicalFit` and
`ScorePreferenceFit` normally; this change is scoped to the gate only.

**Add test coverage** reproducing the Citi/Angular case using the actual
stored hard-exclude label(s) retrieved in Session Start:
- A JD with Angular (or whatever token collided) present only in
  `preferred_skills` and absent from `required_skills` must NOT trip the
  gate.
- A regression check that the same token present in `required_skills`
  *does* still trip the gate — confirming Task 2 didn't disable the check
  entirely, just narrowed its scope to required skills.

---

## Do NOT

- Do not change `JDSignals.RequiredSkills`/`PreferredSkills` from
  `[]string` to a structured type.
- Do not touch `ScorePreferenceFit` or `signalFields` — this session is
  scoped to `ScoreTechnicalFit` and `gateFieldsFor`'s skills handling only.
- Do not retroactively reprocess existing stored `jd_signals` rows — the
  `" | "` convention only affects future extractions. Historical rows with
  flattened OR-lists are a known, accepted inconsistency; not in scope.
- Do not fix the frontend's `required_skills.join(", ")` display to handle
  `" | "` groups more gracefully — cosmetic, and the user has flagged
  frontend polish as separate upcoming work.

---

## Verification Steps

```bash
go build ./...
go test ./internal/fitgate/... -v
go test ./internal/generation/... -v
go test ./...
```

Session is complete when: the two new Task 1 test cases pass, the two new
Task 2 test cases pass, no existing test regresses, and `go build ./...`
is clean.
