# Changelog

Notable changes, newest first. Format loosely follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow
[SemVer](https://semver.org/spec/v2.0.0.html).

Before 0.1.0 this repository was a single-user tool developed in the open. The
git history is complete and the commit messages carry the reasoning; this file
starts at the point the project became something another person could run.

## [Unreleased]

## [0.1.0] — 2026-08-27

First release intended for someone other than its author. Development began
2026-06-10.

### The shape of it

A career knowledge base that generates tailored, traceable resumes. Career
history is stored as discrete contribution "atoms" — richer than any resume
bullet — attached to positions attached to employers. Applying somewhere
queries that data and assembles a resume from the atoms that fit, so choosing
ten bullets out of two hundred is a retrieval problem rather than a
hallucination problem. Every generated bullet carries the contribution ids it
came from.

### Added

- **One-command bring-up.** `docker compose up --build` starts Postgres,
  migrations, the API, the renderer and the web front end. nginx serves the
  bundle and proxies `/api/v1` from the same origin, so the app is correct
  from whatever address you reach it at and the containerised path needs no
  CORS configuration.
- **Career import.** Paste a résumé, a CV or free-form notes; an LLM pass
  stages employers, positions, contributions, skills, preferences, education
  and credentials as drafts you approve one at a time. Nothing reaches your
  record unreviewed, and a draft whose parent is still pending is refused by
  name rather than cascaded.
- **Two LLM stages.** JD signal extraction, then generation split into a body
  call and a summary call scoped to the bullets the body already selected — so
  the summary cannot assert what the resume does not support.
- **A deterministic fit gate.** Capability coverage and stated preferences,
  scored in Go; the model writes the narrative and scores nothing. Preference
  fit is four lists rather than a number, because a number nobody can read back
  to the rows that produced it is what the previous version was.
- **A DOCX renderer** as a separate Python service that never touches the
  database.
- **Career neutrality.** The seniority ladder, the depth scale and the resume's
  section manifest are user-owned rows rather than enums or hardcoded order, so
  the pipeline works for a career that is not software.
  `database/sample-clinical/` is a nurse's career built through the import,
  kept as proof.
- **Two sample datasets**, `make seed-sample` and `make seed-clinical`, so the
  pipeline can be exercised without a career of your own.
- **`make setup`** for the non-container path, and **`make test-all`** across
  all four suites.

### Security

- The server refuses to start on an empty or short `JWT_SECRET`. An empty one
  signs every token with a zero-length key, with no symptom at all.
- Signup is closed by default outside development.
- Rate limiting on the auth routes and on the five endpoints that spend the
  operator's API key; a 2 MB request body cap.
- Timeouts on the HTTP server, the renderer client and the model client,
  ordered so the inner client reports a stall rather than the server killing
  the connection.
- The panic recovery middleware logs the panic and its stack. It previously
  returned a 500 and logged nothing at all.
- See [SECURITY.md](./SECURITY.md), including what is deliberately *not* in
  place.

### Changed

- Personal career material moved to a private repository, and the public git
  history was rewritten to remove it. Prompt blob hashes are content-addressed
  and survived unchanged, so every hash recorded in `generation_params` still
  resolves.
- The whole front end moved onto one theme, and grew a nav shell and a working
  sign-out.

### Fixed

- `CreatePreference` never inserted the `aliases` column, so every preference
  created by an import silently carried none — and the fit gate then matched on
  the label alone.
- The integration suite had never run in CI. It runs now, uncached, and a
  missing environment variable is a failure rather than a green `ok` for a
  package that ran nothing.
- `make seed` could write a real career into the wrong database, including via
  an environment variable that `.env` silently overrode.

### Known gaps

Listed in the README, and short on purpose: no career-data browsing or editing
in the UI, no UI for skills and preferences, no in-app password reset, no
review gate on extracted JD signals, and rendered documents are not persisted.

[Unreleased]: https://github.com/shurikai/role-model/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/shurikai/role-model/releases/tag/v0.1.0
