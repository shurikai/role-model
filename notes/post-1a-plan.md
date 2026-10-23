# Post-1a plan: testers, deployment, design refresh

**Written:** 2026-10-23, after #154/#155/#156 merged and with #162 closing
Phase 1a. Supersedes the phase list in `ROADMAP.md` until that file is updated.
**Companion:** `notes/phase1-career-ui-scope.md` (done; kept for reference).

---

## Roadmap

| Phase | Gate | Contents |
|---|---|---|
| **1a — Import pipeline** | Closes when #162 merges | #158, #159, #161 via #162 |
| **1b — Ready for testers** | A stranger can use the hosted instance, their data is safe, and a failure is visible to me without SSH | Deployment checklist below, design foundations, #157 flaky test, feedback channel |
| **1c — User testing** *(new milestone)* | Two non-technical users enter career data and download a resume without a terminal (the original Phase 1 gate) | Round 1 (current UI), design refresh from findings, round 2 |
| **2 — Output correctness** | Testers trust what comes out | #116, #62, #123, #121, #124, #47 |
| **Later** | | Phase 3 MCP (#125), Phase 4 discovery/Temporal/Kafka, Phase 5 threads, Phase 6 features |

### Moves

- **#120 OIDC: Phase 1b → Later.** Hand-created accounts (`cmd/adduser`) plus
  SSH-side password reset cover testers I've invited. Revisit when people I don't
  know can sign up. If the instance needs to be closed to the internet before
  then, use Cloudflare Access (requires moving DNS off Route 53).
- **#143 remainder: split.** Foundations (tokens, type scale, shared
  components, first-run path) in 1b. Information hierarchy waits for round 1
  findings and lands in 1c.
- **Phase 2 scoring defects → backlog:** #43, #45, #46, #52, #55, #75, #152,
  #153. They come back if a tester hits one.

### Order

1. Merge #162. Close Phase 1a.
2. Deployment checklist items 1–4 (one afternoon).
3. Items 5–7, alongside design foundations.
4. Round 1 with one or two testers on the current UI.
5. Design refresh driven by round 1 findings.
6. Round 2. If two testers complete the core task unaided, the gate is met.
7. Phase 2, ordered by what testers flagged.

**Why test before the full refresh:** what confuses a real person tells the
refresh where to spend effort. Refreshing first is redesigning from guesses.

---

## Deployment checklist (Phase 1b)

Current state: Oracle Always Free A1 VM, `compose.prod.yml` + Caddy, last
deployed 2026-08-31 at `resume.jasonchestnut.net`. The structure is sound; what's
missing is operational coverage for when other people depend on it.

**Protect data (one afternoon)**

- [ ] **1. VM alive and on current main.** Take a backup, then deploy. The
  instance predates the onboarding service, the career endpoints and any
  migrations since Aug 31.
- [ ] **2. Oracle account → Pay-As-You-Go.** Idle Always Free instances can be
  reclaimed; PAYG removes that while staying free within Always Free limits.
  (Verify current Oracle policy.)
- [ ] **3. Backups off the box, plus one restore test.** The cron in
  `docs/deploy-oracle.md` writes to `~/backups` on the same VM. Copy off-box
  (OCI Object Storage free tier or the homelab). Optional: include the
  `role_model_onboarding_checkpoints` volume so in-progress interviews survive a
  rebuild.
- [ ] **4. Separate production Anthropic key with a monthly spend limit.** The
  route-level rate limits don't cap a month's spend.

**See and learn (alongside design work)**

- [ ] **5. Visibility.** Free uptime check on the health endpoint, plus some way
  to see 5xx without SSH (log shipping, or a daily error digest). Testers rarely
  report errors; they leave.
- [ ] **6. Staging.** Second compose project (supported since #115), on the same
  VM or the homelab. Deploy to staging first, then production.
- [ ] **7. Feedback channel.** A link in the shell to a short form or a
  prefilled email. Decide before round 1.

---

## User testing (Phase 1c)

**Round 1**
- 1–2 testers, existing UI, accounts created with `cmd/adduser`.
- Core task: get your career in (any on-ramp), check it on /career, fix one
  thing, paste a real JD, download a resume.
- Watch live or by screen recording. Note where they stall, what they misread,
  what they skip. Don't help unless they're fully stuck, and log every time you do.
- Log each finding as an issue labelled `user-testing`, tagged UX, bug or
  output-quality.

**Before round 1:** re-check the three failures from the first beta pass
(inaccurate import, skills fields throwing errors, one resume failing schema
validation) against current main. Fix or file each.

**Round 2**
- After the refresh. At least one tester new to the app.
- Gate met if two testers complete the core task without help.

---

## GitHub text to paste

**New milestone: Phase 1c — User testing**
> Gate: two non-technical users enter career data and download a resume without
> touching a terminal. Two rounds; findings filed with the `user-testing` label.

**Phase 1b milestone description (replace)**
> Gate: a stranger can use the hosted instance, their data is safe, and a
> failure is visible without SSH. Deployment hardening, design foundations,
> feedback channel.

**New issue: Deployment hardening for user testing** (Phase 1b, `infra`)
> Checklist items 1–7 from `notes/post-1a-plan.md`, as task-list items.

**New issue: Design foundations** (Phase 1b, `frontend`)
> Tokens, type scale, shared components (building on `Page`, `EmptyState`,
> `ConfirmDialog`), and the first-run path for a new account. No
> information-hierarchy changes; those wait for round 1 findings (#143).

**New issue: In-app feedback channel** (Phase 1b, `frontend`)
> A link in the shell to a short form or prefilled email, so testers can report
> problems from where they hit them.

**New issue: Re-verify first beta tester failures** (Phase 1c, `bug`)
> Import inaccuracy, skills fields erroring, one generated resume failing schema
> validation. Reproduce against current main; fix or split into issues.

**Comment on #120**
> Moved out of Phase 1b. Hand-created accounts cover invited testers; revisit
> when signup opens to people outside the test group.

**Comment on #143**
> Split: foundations go to the new Design foundations issue (1b);
> information hierarchy stays here and moves to 1c, driven by round 1 findings.
