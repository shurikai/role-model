# Canonical Context — Role Model Resume Generation

Reference file for per-JD threads. Paste or attach this at the start of any new
chat used to run the fit-assessment / resume-generation pipeline, so the
thread doesn't depend on memory recall alone.

## Hard Rule — Employment Timeline

Never omit any employer. Always include, in this order:

1. Lockheed Martin (Jun 1999 – Feb 2016)
2. Manifold Technology (2016 – 2018, RBC satellite office, **Orlando, FL** — not Toronto)
3. Dignitas Technologies (2018 – 2020)
4. MAK Technologies (Jun 2020 – Oct 2020)
5. AEMWAS — American Electronic Warfare Associates (Jan 2021 – Feb 2022)
6. Daugherty Business Solutions (Feb 2022 – Nov 2024)
7. Disney Cruise Line (Sep 2025 – Apr 2026, most recent)

**Pelotech** (~4 weeks, 2025, TypeScript/Node.js consulting) is the only
position that may be omitted by default — but must be included if the JD
requires a complete employment history, or if TypeScript/Node.js is a JD
requirement. Framing: "a short consulting engagement that turned out not to be
the right fit."

## Canonical Facts

- **Employment dates** (confirmed accurate): Lockheed Martin Jun 1999 – Feb 2016;
  Manifold Technology 2016 – 2018; Dignitas Technologies 2018 – 2020;
  MAK Technologies Jun 2020 – Oct 2020; AEMWAS Jan 2021 – Feb 2022;
  Daugherty Business Solutions Feb 2022 – Nov 2024; Disney Cruise Line Sep 2025 – Apr 2026.
- Lockheed's IOS product was a **C#/.NET Windows desktop application**
  (relevant when a JD calls for Windows desktop or C# experience).
- **macOS** is daily dev platform, with professional relevance.
- Manifold Technology location is **Orlando, FL** (RBC satellite office) —
  never Toronto, ON.
- **Java**: expert, 25 years. Primary language throughout career.
- **Go**: active, publicly verifiable (dedupe and Role Model on GitHub at
  github.com/shurikai). Daugherty GCP cohort prototype = professional Go signal.
  "2+ years professional Go" is an honest gap if a JD requires it explicitly.
- **Groovy**: picked up in under a week at Manifold for a special project —
  the canonical "fast language ramp" proof point.
- **C# / .NET**: Lockheed IOS product (Windows desktop). Not daily use;
  actively disinterested in C# as a primary language.
- **Python**: thin signal (AEMWAS pipelines, Daugherty cohort). Not primary.
- **Microservices**: confirmed strength — Disney Cruise Line Navigator app
  backend is a microservices architecture; both Daugherty engagements involved
  monolith-to-microservices decomposition.
- **Customer-facing work**: Jason was effectively in an FDE-style role at
  Manifold (embedded at RBC customer site). Regular direct customer interaction
  also common at Lockheed Martin. Sales-facing work is NOT a strength.
- **Docker/Jenkins**: confirmed legitimate skills across multiple employers
  plus active homelab.
- **Mentoring**: consistent strength across virtually every employer.
- **Kafka**: production experience at MAK Technologies (C++ plugins, real-time
  simulation telemetry).
- **Cassandra**: production experience at Manifold Technology.
- **DIS/dead-reckoning**: the canonical bridge for fleet/IoT/physical operations
  roles. Real-time vehicle state coordination across distributed nodes.
- **AI tooling**: Claude Code, GitHub Copilot, Cursor — daily production use.
  Honest characterization: "daily tool use + one architected pipeline, not
  production ML-ops at scale."
- **Role Model**: Go backend (chi, pgx, sqlc), PostgreSQL, two-stage Anthropic
  LLM pipeline. Designed with Temporal as intended orchestration layer.
  Public at github.com/shurikai/role-model.
- **dedupe**: Go CLI for photo deduplication, Charm/Bubble Tea TUI.
  Public at github.com/shurikai/dedupe.
- **Education**: B.S. Computer Science, Mathematics minor, Tulane University, 1999.
- **Location**: Orlando, FL. Phone: (407) 491-5684.
  Email: jason.chestnut@gmail.com.
- **GitHub**: github.com/shurikai. **Blog**: shurikai.github.io.
- **LinkedIn**: linkedin.com/in/jason-chestnut.
- Avoid em-dashes in prose sections (LLM tell).

## Formatting Standards (Node.js/docx generation)

All resumes use Arial font, accent color #1A5276.

### Section headings
```js
spacing: { before: 320, after: 20 }
```

### Rule after heading
```js
spacing: { before: 0, after: 120 }
```

### Tab stops (employer/date and education/year lines)
Split into THREE separate TextRun objects — do not embed tab in a string:
```js
new TextRun({ text: company, bold: true, size: 22, font: "Arial", color: BLACK }),
new TextRun({ text: "\t", size: 22, font: "Arial" }),
new TextRun({ text: tenure, size: 20, font: "Arial", color: GRAY })
```
Same pattern for education line (university name / tab / year).

### Right tab stop position
```js
tabStops: [{ type: TabStopType.RIGHT, position: CONTENT_WIDTH }]
// where CONTENT_WIDTH = PAGE_WIDTH_DXA - MARGIN_DXA * 2 = 10080
// PAGE_WIDTH_DXA = 12240, MARGIN_DXA = 1080
```

### Page margins
```js
margin: { top: 1080, right: 1080, bottom: 1080, left: 1080 }
```

### Bullet indentation
```js
style: { paragraph: { indent: { left: 480, hanging: 280 } } }
```

### Employer line spacing
```js
spacing: { before: 140, after: 20 }
```

### Title line spacing
```js
spacing: { before: 0, after: 60 }
```

### Bullet spacing
```js
spacing: { before: 20, after: 20 }
```

## Job Search Targeting (for fit assessment)

**Target domains:** distributed systems, IoT/telemetry, real-time data,
consumer-facing product, physical operations, observability

**Work type:** product over platform, small-team high-ownership, greenfield
over pure maintenance, remote-first (also open to onsite in Orlando, FL)

**Culture:** low-ego, async, not military-coded

**Hard excludes:**
- Big Four consulting culture
- Pure frontend or frontend-coequal full-stack
- Defense/aerospace (BlackSky is a noted last-resort fallback given clearance history)
- AI-production-required roles without real signal (LangGraph, CrewAI, RAG at scale, etc.)
- Internal platform/DevEx roles (customers are internal engineers, not end users)
- Principal-level roles (generally out of range; one above Staff target)
- Ruby/Rails primary (actively disinterested; shrinking market share)
- C# / .NET as daily primary language (actively disinterested)
- Node.js / TypeScript as primary stack
- Python-primary requiring expert depth
- Crypto/blockchain (values)
- Onsite outside Orlando, FL
- Anonymous postings with no named employer (flag; may proceed but note risk)

**Recurring honest gaps:** Redis, Terraform, gRPC, production AI/ML features,
payment rails/ACH, TypeScript/Node.js primary, Python-primary at expert level,
Go "2+ years professional" claims, Kubernetes deep operational ownership.

## Stage 1 / Stage 2 Pipeline

**Stage 1 — Fit assessment:**
- Extract required vs. preferred qualifications
- Check against hard-exclude list
- Identify language/stack alignment
- Flag domain gaps honestly
- Stop here if hard excludes apply (no resume generated)

**Stage 2 — Honest gap identification:**
- Name specific gaps by technology/domain
- Distinguish "honest bridge" from "actual gap"
- Confirm proceed before generating

**Generation:**
- Node.js script using docx library
- Output to /mnt/user-data/outputs/Chestnut_Jason_[Company].docx
- Validate with office/validate.py after generation

## Pipeline Steps (manual, per JD)

1. Paste or upload JD (use .text extension if Safari/clipboard issues).
2. Stage 1 — fit assessment against canonical story + hard-exclude check.
3. Stage 2 — honest gap identification.
4. Generate tailored resume via Node.js script.
5. Present file to user.

---
*Source of truth is the structured career record (Role Model DB), not this
file or any resume output. This file exists only to keep per-JD threads
short and self-contained.*
