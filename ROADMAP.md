# Role Model — Roadmap

**Last updated:** August 2026
**Repo:** github.com/shurikai/role-model
**Data repo:** private, accessed via `SEED_DIR` env var from `make seed`
**Companion docs:** `CLAUDE.md`, `notes/discovery-design.md`,
`notes/role-model-schema-design.md`

> **Which document is which.** [`README.md`](./README.md) is for someone
> deciding whether to run this and how. [`CLAUDE.md`](./CLAUDE.md) is the
> conventions document — stack, architecture, and the rules that hold, most of
> them written because the alternative was tried and cost something. This file
> is the phase map and the positioning statement. **Planned work is tracked in
> GitHub Issues, grouped by milestone — one milestone per phase.** The
> per-item detail and design rationale live in the issues, not here. Where this
> file disagrees with `CLAUDE.md`, `CLAUDE.md` is current.

---

## System Identity

Role Model is a self-hostable, AI-powered career management platform. Its core
thesis: a person's career history, structured as a queryable knowledge base of
contribution "atoms," is the raw material for all downstream job search
activities — resume generation, fit assessment, cover letters, interview prep.

The LLM layer synthesizes and personalizes. SQL retrieves and filters. These
roles are not interchangeable, and the architecture enforces that separation.

This is bespoke RAG: deterministic SQL retrieval driven by Stage 1 LLM signal
extraction. The resume generation pipeline is sequential, human-in-the-loop, and
deterministic by design. The LLM's role in generation is synthesis from
structured data, not autonomous decision-making.

The data-entry surface is a different story. An agent-driven onboarding
interview (see Phase 1) uses a multi-step, tool-calling agent framework to walk
users through populating their career data conversationally. Agent patterns are
appropriate here because the task is exploratory and conversational; they remain
inappropriate for the generation pipeline itself, where auditability and
determinism are the point.

**Positioning:** personal career knowledge base, not a resume generator.
The distinction matters for retention (value compounds over time), for honest
product framing, and for any future monetization argument (the structured data
is the moat, not the prompts).

---

## Phases

Each phase is a GitHub milestone. The **gate** is its exit criterion. Follow
the milestone link for the live issue list; the "open work" below is a snapshot
for orientation.

| Phase | Gate | Milestone |
|---|---|---|
| **Phase 1** — Usable by humans other than me | Two non-technical test users can enter career data and get a resume without touching the terminal | [milestone](https://github.com/shurikai/role-model/milestone/2) |
| **Phase 2** — Complete end-to-end pipeline | JD input → `.docx` download without leaving the UI *(met; remaining work is review gates, feedback, correctness eval)* | [milestone](https://github.com/shurikai/role-model/milestone/3) |
| **Phase 3** — MCP Server | Career data queryable by any MCP client through live tools, not a derived copy | [milestone](https://github.com/shurikai/role-model/milestone/4) |
| **Phase 4** — Discovery and distributed systems | New postings from target companies appear automatically; the pipeline runs durably end to end | [milestone](https://github.com/shurikai/role-model/milestone/5) |
| **Phase 5** — Career threads | Named narrative through-lines, system-proposed and human-confirmed, feed summaries and fit scoring | [milestone](https://github.com/shurikai/role-model/milestone/6) |
| **Phase 6** — Richer features | At least one non-me user finds the system genuinely useful over time | [milestone](https://github.com/shurikai/role-model/milestone/7) |

What is already built and verified is in `README.md` `## Status`; the schema of
record is `migrations/` and `notes/data-model.md`.

### Phase 1 — Usable by humans other than me

Gate: two non-technical test users can enter career data and get a resume
without touching the terminal.

Open work: onboarding agent (#117), career-data browsing/editing UI (#118),
Stage 0 import review UI (#119), OIDC login (#120), Oracle Cloud deployment
(#20), Stage 0b tag suggestions (#19), Stage 0b tag-suggestion review UI (#137).

### Phase 2 — Complete end-to-end pipeline

Gate met — a pasted JD reaches a downloaded `.docx` in the browser. What
remains is the review-and-correctness layer.

Open work: review gate on extracted `jd_signals` (#121), fit-report corrections
(#122), three-pass output reviewer (#123), Stage 2 generation-correctness eval
(#124), Stage 1 extraction eval Layer B (#47), feedback loop endpoint (#9),
project contributions seed (#22), `applied_on` date parsing (#6). Plus a
cluster of known fit-gate and generation correctness defects: #43 #45 #46 #52
#55 (scoring), #75 (term matching), #62 (summary length rules), #51 (eval
fixture drift guard).

### Phase 3 — MCP Server

Gate: career data is queryable by any MCP client through live parameterised
tools, not a derived copy. Not a phase proper — infrastructure that depends on
Phase 2 and is depended on by later work.

Open work: MCP server (#125).

### Phase 4 — Discovery and distributed systems

Gate: new postings from target companies appear automatically; the pipeline
runs durably end to end. Kafka owns continuous-world fan-out (discovery →
independent consumers); Temporal owns the single application's durable,
human-gated journey.

Open work: discovery worker (#12), Temporal integration (#13), Kafka event
pipeline (#126), notification consumer + channel decision (#127), application
status / CRM pipeline (#128).

### Phase 5 — Career threads

Gate: named narrative through-lines, system-proposed and human-confirmed, feed
resume summaries and fit scoring. Can be built alongside or after Phase 4;
depends on career data being well-seeded.

Open work: career threads (#129).

### Phase 6 — Richer features

Gate: at least one non-me user finds the system genuinely useful over time.

Open work: cover letter generation (#130), interview prep (#131), outcome
tracking (#132), fit-gate calibration against outcomes (#133), prompt-quality
evaluation framework (#11), blob storage interface (#10).
