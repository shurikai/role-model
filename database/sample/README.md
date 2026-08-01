# Sample dataset

Everything in this directory is **fictional**. Morgan Reyes does not exist,
neither do Continental Freightways, Palletwise, or Northbound Logistics, and
none of the accomplishments described here happened. The dataset exists so
that someone who clones this repository can run the full pipeline — fit gate,
generation, render — and see real output without access to a private
career-history seed set.

Real career data lives outside the repo in `$SEED_DIR` and is gitignored. See
the README section "Career data is seeded, not written through the API."

## Loading it

```sh
make db-up
make migrate-up
make seed-sample        # loads database/sample into $DATABASE_URL
```

Then log in as `sample@example.com` / `sample-password`.

`make seed-sample` is deliberately a separate target from `make seed` rather
than a fallback default for `SEED_DIR`. An absent-minded `make seed` must
never inject invented employers into a database holding real career history.

**Point it at a scratch database.** If your `DATABASE_URL` holds real data,
override it:

```sh
make seed-sample DATABASE_URL="postgres://user:pass@localhost:5433/role_model_sample?sslmode=disable"
```

Every file uses explicit stable UUIDs with `ON CONFLICT ... DO UPDATE`, so
re-running converges rows rather than duplicating them. All UUIDs here begin
with `5` (for "sample"), so they cannot collide with a real seed set loaded
into the same database.

## Files

| File | Contents |
|---|---|
| `001_foundation.sql` | user, employers, positions, tag_categories, tags |
| `002_continental_freightways.sql` | contributions + contribution_tags (2013–2017) |
| `003_palletwise.sql` | contributions + contribution_tags (2017–2021) |
| `004_northbound_logistics.sql` | contributions + contribution_tags (2021–present) |
| `005_projects.sql` | projects, project_tags, **project_contributions** |
| `006_education_credentials.sql` | education, education_tags, credentials, **credential_tags** |
| `007_skills_preferences.sql` | **skills**, **preferences** |

The bolded tables are populated by **no file in the private seed set**. That
is the main reason this dataset is worth having beyond onboarding: it is the
only place several code paths get exercised at all.

- `skills` — migration 008 backfills this from `contribution_tags`, but on a
  fresh clone that migration runs against empty tables. Without
  `007_skills_preferences.sql` the table stays empty after seeding and
  generation loses its skill signal entirely.
- `preferences` — `internal/fitgate` scores preference fit against this table.
  With no rows the fit gate only ever produces its technical half.
- `project_contributions` — issue #22. Projects that link back to the work
  that produced them let generation connect the two.
- `credential_tags` — the credential→tag path is otherwise untested.

`v_skill_provenance` needs no seeding: it is a view derived from `skills` and
`contribution_tags`, and populates automatically (146 rows with this dataset).

## Skill depth

Proficiency in `007_skills_preferences.sql` is deliberately **varied**, and
that is the point. Migration 008 backfilled the real dataset at a uniform
`proficient` / `NULL` years, so a weekend prototype and a decade of production
work are indistinguishable to generation.

This sample is the one dataset where depth actually differs, including the
case that matters most: skills with many years but only moderate depth (SQL
and REST, 13 years each, `proficient`) versus fewer years and real depth (Go,
9 years, `expert`). Two skills are marked `is_active = FALSE` — Spring Boot
and Jenkins — so the active-skill filter has something to filter.

## Paired JD fixtures

Three fixtures in `tests/fixtures/` are written against this persona. Without
them the fit gate scores near zero on any existing JD and the tool looks
broken rather than correctly reporting a bad fit.

| Fixture | Exercises |
|---|---|
| `sample-strong-match-jd.md` | High technical coverage and high preference fit. Staff backend role in freight visibility: Go, Kafka, PostgreSQL, Kubernetes, remote-first, explicit mentorship, IC track to Principal. |
| `sample-weak-match-jd.md` | Both preference failure modes at once, kept distinct. Frontend-majority fintech role, hybrid onsite, on-call heavy: unmatched positive preferences (logistics, backend, remote) surface as **gaps**, while matched negative preferences (frontend, on-call heavy, mandatory overtime) surface as **conflicts**. Technical coverage is partial rather than zero — React and TypeScript do match, at `novice`. |
| `sample-hard-exclude-jd.md` | The anti-pattern gate. An adtech RTB role that is a near-perfect *technical* match (Go, Kafka, PostgreSQL, Redis, AWS, Kubernetes, remote) and still fails the gate outright on the `adtech` hard exclude. Deliberately attractive on every other axis, because a gate that only ever fires on obviously bad roles proves nothing. |

Keeping gaps and conflicts separate is a standing constraint — collapsing them
produces false conflict language. The weak-match fixture is what catches a
regression there.
