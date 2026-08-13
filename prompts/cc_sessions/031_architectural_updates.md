# Session 031 — ROADMAP.md Architectural Updates

## Objective

Update ROADMAP.md to reflect architectural decisions made in design
conversations since the document was last updated. Session 030 handled
mechanical state drift (file counts, paths, closed items). This session
adds new sections and revises existing ones to reflect decisions about
MCP, agent frameworks, eval, and job-search targeting.

**Important:** These are decided directions, not proposals. Apply them as
documented. If anything here contradicts the current ROADMAP text, the
direction stated here wins.

## Pre-flight

```bash
git pull
```

Confirm session 030 has already been applied (seed file count should be 19,
companion docs path should reference `notes/discovery-design.md`). If not,
apply 030 first.

Read the full current ROADMAP.md before making any changes.

## Instructions

### 1. Revise System Identity

The current text says:

> LangChain-style orchestration, the Claude Agent SDK, and general agentic
> loop patterns are all explicitly out of scope — the pipeline is sequential,
> human-in-the-loop, and deterministic by design.

Replace with language that preserves the core pipeline's non-agentic,
deterministic character while acknowledging that the data-entry surface is
evolving to include agent-driven workflows. Something like:

> The resume generation pipeline is sequential, human-in-the-loop, and
> deterministic by design. The LLM's role in generation is synthesis from
> structured data, not autonomous decision-making.
>
> The data-entry surface is a different story. An agent-driven onboarding
> interview (see Phase 1) uses a multi-step, tool-calling agent framework
> to walk users through populating their career data conversationally. Agent
> patterns are appropriate here because the task is exploratory and
> conversational; they remain inappropriate for the generation pipeline
> itself, where auditability and determinism are the point.

Preserve the existing paragraph about positioning (personal career knowledge
base, not a resume generator).

### 2. Add MCP Server section

Add a new section at the Phase 2 level or between Phase 2 and Phase 3
(use your judgment on placement — it's infrastructure that supports
multiple phases). Title: "MCP Server — Protocol Surface for Career Data"

Key points to include:

- The MCP server exposes Role Model's existing career data as MCP tools,
  NOT as a static resource serving `canonical-context.md`. The whole point
  of SQL-as-retrieval is that queries are live and parameterized.
  `canonical-context.md` is a derived, third-generation copy used for
  bootstrapping chat threads — it has no role in the MCP surface.
- Implementation: thin layer in front of the existing Go backend. Same
  pgx/sqlc queries the pipeline already uses, exposed as MCP tools.
- Process boundary is an open question: separate process alongside the chi
  API (needs its own pgx pool, decides whether to import the same sqlc code
  or duplicate it), or embedded as a second protocol surface in the same
  binary (cleaner code sharing, changes deployment shape). Note this as a
  decision to make during implementation, not now.
- Transport: Streamable HTTP (the current MCP spec standard as of
  2026-07-28; HTTP+SSE is deprecated).
- Candidate tools (initial surface, not exhaustive):
  - `search_contributions(query, tags?, employer?, limit?)` — the core
    retrieval primitive, live against Postgres
  - `get_skill(skill_name)` — returns status, proficiency, supporting
    contributions, and honest framing notes
  - `list_employment_timeline()` — the canonical never-omit timeline
  - `check_hard_excludes(jd_signals)` — Stage 1 gate as a callable tool
  - `get_gap_analysis(required_skills[])` — Stage 2 gap identification
- Resources (MCP primitive, read-only addressable data) may have a role for
  stable reference data like the tag taxonomy or anti-pattern list, but
  career facts themselves must be tools, not resources.
- Relationship to Temporal: if the pipeline eventually gets orchestrated by
  Temporal workflows, MCP tools need to decide whether they trigger Temporal
  workflows or stay below that layer. Note as an open question.

### 3. Add Onboarding Agent to Phase 1

Stage 0 (batch import) is already built and documented. Add the onboarding
agent as a **parallel data-entry path**, not a replacement for Stage 0.

Key points:

- Stage 0 handles the "paste a resume or LinkedIn export" case. The
  onboarding agent handles the case where a user doesn't have a clean
  document to start from, or whose career doesn't decompose neatly into
  pre-written bullets.
- The onboarding agent is a conversational, multi-step, tool-calling agent
  built with LangGraph. Framework decision is final — do not substitute
  CrewAI or another framework. Rationale: the task is a single structured
  interview with conditional branching and state, not multi-agent
  collaboration, which matches LangGraph's graph-of-nodes model rather than
  CrewAI's crew/role abstraction. LangGraph's checkpointing/interrupt
  pattern also mirrors the Temporal Signal-based human-in-the-loop design
  already planned for Phase 3, so the mental model transfers. It conducts a
  structured interview modeled on a recruiter/career-coach conversation.
- It calls Role Model's existing REST API endpoints to write contributions
  as the conversation progresses. It does not bypass the API or write
  directly to Postgres.
- This is also a deliberate skill-acquisition project: hands-on experience
  with a production agent framework, building a real feature rather than a
  throwaway spike. This supports the FDE career track (see Hard-Pass
  Filters update below).
- The eval story for v1 is self-referential: run it against the existing
  career data (where the right answers are known) and evaluate whether it
  asks good questions and writes accurate contributions.
- Design principles (carried from earlier design discussion):
  1. Trust boundary is structural honesty, not fact-checking — the system
     enforces that every skill claim links to a supporting contribution,
     but does not verify underlying claim truth.
  2. The interview produces the field's tag vocabulary as a byproduct,
     which is cheaper than pre-building vocabulary per field.
  3. Sequencing: get base functionality + frontend solid first, validate
     against a second field (e.g. education) before designing for many
     fields speculatively.

### 4. Add Eval Harness section

Add to Phase 2 or create a new subsection. This is distinct from the
existing Phase 4 "Prompt evaluation framework" bullet (which is about
comparing prompt versions). The eval harness is about correctness
verification of the pipeline's outputs.

Key points:

- Fixture set of JDs with known-correct signal extractions, scored
  automatically. The test asserts that Stage 1 extracts the right signals
  from a known JD, not just that it produces valid JSON.
- Extends to Stage 2: given known signals and known career data, does
  generation select the right contributions and produce accurate bullets?
- This is also an FDE credibility artifact: "I built and evaluated a
  production agentic system" is the sentence FDE screens are listening for
  in 2026, and the eval harness is what makes it true.
- Note that `tests/fixtures/` already exists for pipeline regression
  fixtures — the eval harness builds on this, not alongside it.

### 5. Update Hard-Pass Filters

The current "Production LLM/AI-feature experience as a hard requirement"
line needs to be replaced with the three-category breakdown:

1. **FDE-shaped roles** (deploying/operating agents for a customer's real
   business problem): conscious exception, evaluated on its own track.
   Bridge story: Manifold/RBC embedded customer work + Lockheed direct
   customer interaction.
2. **Agent orchestration/runtime/framework platform roles** (deliverable is
   the agent-building infrastructure itself, customers are other engineers):
   remains excluded on values grounds (product-over-platform), not just
   skill grounds.
3. **"AI-production-required" as a qualifications-line technicality** on an
   otherwise-normal role: softened from blanket Stage 1 auto-fail to Stage 2
   case-by-case judgment, gated on MCP server + eval harness shipping.

Also add a note that learning agent frameworks (LangGraph/CrewAI),
multi-agent coordination patterns, and building the Temporal integration
are skill-acquisition moves that support category 1. The exclude is scoped
to who consumes the output, not to any particular technology.

Update the "defense / aerospace" line to "defense-coded / clearance-required"
if session 030 didn't already handle this.

### 6. Update Known Skill Gaps table

Add a row for agent frameworks:

| Agent frameworks (LangGraph) | No hands-on experience yet; onboarding agent (Phase 1) is the planned closure path — LangGraph specifically, decision final |

### 7. Add Temporal to Phase 3 context

Temporal integration is already documented in Phase 3, but add a note that
it also serves the FDE credibility story: Temporal is general
distributed-systems orchestration infrastructure (not AI-agent technology),
and building it demonstrates the DIS/dead-reckoning distributed-coordination
background applied to a new domain.

## What NOT to change

- Do not reorganize the phase structure (Phase 1/2/3/3.5/4). New items
  slot into existing phases.
- Do not change the Stack table.
- Do not modify the Implementation Checkpoint section (that was session 030).
- Do not add the company research brief or any other features not listed
  above — this session is scoped to the MCP/agent/eval/targeting decisions.
- Do not remove the original skills/preferences schema proposals that are
  retained "for contrast" — they serve as design history.

## Validation

After making changes, read the full ROADMAP.md and confirm:
- System Identity no longer claims agentic patterns are blanket out-of-scope
- MCP server section exists and describes tools-over-API, not
  canonical-context.md-as-resource
- Onboarding agent is documented as a parallel path to Stage 0, not a
  replacement
- Eval harness is documented as a correctness tool, distinct from the
  Phase 4 prompt-version comparison
- Hard-pass filters reflect the three-category AI breakdown
- No section contradicts another

## Commit

```bash
git add ROADMAP.md
git commit -m "docs: add MCP server, onboarding agent, eval harness, update AI targeting categories"
```
