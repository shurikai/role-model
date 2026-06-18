# Discovery Worker — Design Sketch

Status: deferred — parked until Stage 2 (resume generation) pipeline is wired as an
API endpoint. This document exists so the idea can be picked back up cold.

## Why this exists

Role Model is currently driven by manually feeding job descriptions into the
Stage 1 / Stage 2 pipeline. The discovery worker automates the *first* step:
finding job postings worth feeding in, by polling the public job-board APIs of
specific target companies (Grafana, Temporal, HashiCorp, Honeycomb, etc.)
rather than scraping HTML or aggregator sites like LinkedIn/Indeed (ToS risk,
brittle, not the employer's own data).

This is also the part of the system that justifies an event bus (Kafka or
similar). The discovery worker is a genuine asynchronous producer — no caller
waiting on a response, runs on its own schedule — and it can have more than
one real consumer (extraction, notification, future dedup/filtering). That
multi-consumer fan-out is what makes Kafka an honest choice here, as opposed
to bolting it onto a single-consumer step elsewhere in the pipeline. See
prior discussion: the resume-generation pipeline itself (signal extraction →
human review → generation → render) is better modeled as a Temporal workflow,
since it's sequential, durable, and human-in-the-loop by design. Discovery
feeds *into* that workflow; it doesn't replace it.

## Decisions made

- **Location**: `cmd/discovery` inside the existing `role-model` repo, not a
  separate repo. Shares `go.mod`, so the `JobPosting` struct and Kafka event
  types are defined once and used by both the producer (discovery) and
  consumer (extraction) sides without cross-repo version drift. Runs as its
  own process (`make run-discovery`), so it has its own failure domain at
  runtime even though it lives in one module. Splitting into a separate repo
  later is mechanical if independent deploy/scaling ever becomes a real need
  — the Kafka boundary already enforces decoupling at the architecture level,
  not just the repo level, so this is a low-regret choice to defer.
- **Config**: YAML file (`companies.yaml`), not a database table. A restart
  to pick up a new target company is trivial for a single-user, self-hosted,
  low-frequency-change list. Revisit only if this needs to be editable
  without a deploy (e.g. if it ever becomes multi-tenant).
- **Targets confirmed to have public, unauthenticated, JSON-returning job
  feeds** (verified against current docs, June 2026):
  - **Greenhouse**: `GET https://boards-api.greenhouse.io/v1/boards/{token}/jobs?content=true`
    — confirmed working for `grafanalabs` and `temporaltechnologies` board
    tokens specifically.
  - **Ashby**: `GET https://api.ashbyhq.com/posting-api/job-board/{clientname}?includeCompensation=true`
    — this is Ashby's intentionally public, no-auth feed (distinct from their
    authenticated developer API, which is for the employer's own use).
    Cleanest of the three: includes a ready-to-use `descriptionPlain` field,
    no HTML stripping needed.
  - **Lever**: `GET https://api.lever.co/v0/postings/{company}?mode=json` —
    same family of public feed, not yet individually verified against a
    specific target company's slug. Verify before building the adapter.
  - **Not every target will have one of these.** GitHub did not surface a
    standalone Greenhouse/Lever board in search — it likely runs under
    Microsoft's own (probably non-public-API) careers system post-acquisition.
    The adapter design needs a graceful "no structured source available, skip
    or hand-roll" path rather than assuming every company fits the pattern.

## Adapter interface

One interface, one implementation per ATS platform. Each adapter normalizes
its platform's quirks into the same `JobPosting` shape so nothing downstream
needs to know which platform a posting came from.

```go
package discovery

import "time"

// JobPosting is the normalized shape every adapter produces, regardless of
// source platform. This is also the payload embedded in the
// job_posting.discovered Kafka event.
type JobPosting struct {
    SourcePlatform   string    // "greenhouse" | "ashby" | "lever"
    SourceCompany    string    // config identifier, e.g. "grafanalabs"
    ExternalID       string    // platform's stable job ID, for de-duplication
    Title            string
    DescriptionText  string    // plain text, ready for Stage 1 extraction
    Department       string    // best-effort; not all platforms provide this cleanly
    Location          string
    Remote           bool
    URL              string    // canonical posting URL, for the human reviewing later
    PublishedAt      time.Time
    DiscoveredAt     time.Time // when *this worker* first saw it, not when it was posted
}

// Adapter is implemented once per ATS platform.
type Adapter interface {
    // Platform returns the adapter's platform identifier, used in config
    // and in JobPosting.SourcePlatform.
    Platform() string

    // Fetch returns all currently listed postings for one configured
    // company. Adapters are responsible for their own HTTP client setup,
    // pagination if the platform requires it, and mapping platform-specific
    // fields onto JobPosting. Adapters do NOT decide what's "new" — that's
    // the poller's job, by diffing ExternalID against what's already been
    // seen.
    Fetch(ctx context.Context, companyIdentifier string) ([]JobPosting, error)
}
```

Each adapter (`GreenhouseAdapter`, `AshbyAdapter`, `LeverAdapter`) is a small,
independently testable struct implementing this interface. None of them need
to know about Kafka, the poller, or config — they just turn one HTTP response
into `[]JobPosting`. That separation is what makes integration tests for each
adapter straightforward (recorded HTTP fixtures, no live network needed).

## Config shape — `companies.yaml`

```yaml
companies:
  - name: "Grafana Labs"
    platform: greenhouse
    identifier: grafanalabs
    poll_interval: 30m

  - name: "Temporal Technologies"
    platform: greenhouse
    identifier: temporaltechnologies
    poll_interval: 30m

  - name: "Some Ashby Company"
    platform: ashby
    identifier: someclientname
    poll_interval: 1h
```

Per-entry `poll_interval` allows tuning frequency per company without code
changes — e.g. higher-priority targets polled more often. The poller loads
this once at startup; a config change requires a restart, which is the
accepted tradeoff for YAML over a DB table.

## What "new" means — de-duplication

The poller, not the adapters, owns the question of whether a posting is new.
Simplest viable approach: a small table (or even a flat file/KV store to
start) recording `(source_platform, source_company, external_id) → first_seen_at`.
On each poll cycle, fetch current postings, diff against known IDs, emit
`job_posting.discovered` only for ones not already recorded, then record them.

This will eventually want to live in Postgres alongside the rest of Role
Model's schema (a `discovered_postings` table), since the discovery worker
and the main API already share a database in the single-host deployment. No
need to invent separate storage for this.

## Event: `job_posting.discovered`

Published to a Kafka topic (e.g. `job-postings.discovered`) once per newly
seen posting. Payload is the `JobPosting` struct, JSON-encoded.

Known/likely consumers, to validate this is genuine fan-out and not a queue
with one listener wearing a costume:

1. **Extraction consumer** — picks up the event, runs Stage 1 (JD signal
   extraction) against `DescriptionText`, creates the `applications` row,
   publishes `jd_signals.extracted` once done. This is the consumer that
   hands off into the Temporal-modeled generation workflow.
2. **Notification consumer** — pings you (however you want to be notified —
   revisit channel choice later) either on raw discovery or, more usefully,
   on `jd_signals.extracted` so you're notified with a fit assessment
   already attached rather than a bare posting.
3. **(Future, optional) Dedup/filtering consumer** — could subscribe to the
   same `job_posting.discovered` topic independently to flag likely
   duplicates or low-fit postings before they reach extraction, without
   extraction or the scraper needing to know it exists. Not needed at launch;
   noted here so the topic design doesn't accidentally preclude it.

## Explicitly out of scope for v1

- Scraping any site without a public JSON feed (no HTML parsing path planned;
  if a target company doesn't have one of the three supported platforms,
  it's manually handled, not automated).
- Any write-back to the ATS platforms (this is read-only discovery).
- Multi-tenant config (per-user company lists) — not needed for a
  single-user system; the `user_id` precedent elsewhere in the schema covers
  this if it's ever revisited.
- Real-time push/webhooks — polling on an interval is sufficient for this
  use case and avoids needing a publicly reachable inbound endpoint.

## Open questions for when this is picked back up

- Confirm the Lever public feed shape against a real target company before
  writing that adapter — only Greenhouse and Ashby have been verified above.
- Decide notification channel (email? a simple webhook to a phone
  notification service? something else already in use?) before building the
  notification consumer.
- Decide where `discovered_postings` (de-dup tracking) lives relative to
  existing migrations — likely just another table in the same Postgres
  instance, via the normal golang-migrate flow.
- Revisit whether `cmd/discovery` should eventually move to its own repo once
  there's an actual deployment reason (independent scaling, independent
  on/off without touching the API binary) — not before.
