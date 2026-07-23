# Session NNN: Application Generation Flow (JD → Fit → Generate → Render)

## Goal

Build the minimal frontend flow that lets Jason run a real job description
end-to-end without touching a script or raw SQL by hand: paste a JD, see
Stage 1 signals and the fit report, trigger Stage 2 generation, and download
the rendered resume once the Python renderer is reachable.

This is one of two co-equal frontend gaps blocking real usage (the other is
a contributions CRUD editor — separate session, not in scope here).

## Scope

**In scope:**

- New route: **Create Application** — paste JD text, submit, triggers Stage 1
- **Application detail view** — shows `jd_signals` and the fit report
  (hard-gate pass/fail, technical score, preference score, gaps)
- **Generate action** — triggers Stage 2 for an application that passed the
  fit gate; displays the resulting `resume_version`
- **Render/Download action** — calls `POST /resume-versions/{id}/render`,
  handles the binary response as a browser file download
- **Applications list view** — so Jason can navigate back to prior runs and
  see status at a glance (fit pending / passed / failed / generated /
  rendered)

**Out of scope — do not build this session:**

- Contributions CRUD editor UI (next session, equal priority, separate scope)
- Stage 0 import review UI
- Styling polish beyond functional Tailwind consistent with the existing
  auth shell components
- OAuth, Fly.io deploy, three-pass reviewer, career threads, Temporal —
  none of this is relevant to this session

## Relevant existing code / patterns to follow

- `lib/api-client.ts` — zero React imports, global unauthorized-handler
  registration pattern (`setUnauthorizedHandler`)
- `contexts/AuthContext.tsx` — background `/auth/me` verification via
  TanStack Query, `staleTime: Infinity`, `refetchOnWindowFocus: false`,
  never gates render
- `components/RequireAuth.tsx` — reuse this auth-gating pattern for all new
  routes added in this session
- `lib/session.ts` — localStorage wrapper under the `role_model_session` key
- `lib/types.ts` — existing TypeScript response typing conventions to follow
  for the new DTOs (applications, jd_signals, fit_report, resume_version)

## Backend routes to verify before wiring

Do not assume exact route names/methods from memory — confirm each against
`router.go` before building the corresponding frontend call:

- Application creation / Stage 1 trigger (holds `jd_signals` as JSONB)
- Fit gate: `/applications/{applicationID}/fit`
- Stage 2 generation endpoint
- `resume_versions` resource (versioned, immutable)
- `POST /resume-versions/{id}/render` — exists as a route stub per the
  original API contract but is not yet wired to a real renderer call; this
  session wires it

## Deliverables

1. `routes/Applications.tsx` — list view with per-application status
2. `routes/ApplicationNew.tsx` — JD paste form → Stage 1 → redirect to detail
3. `routes/ApplicationDetail.tsx` — `jd_signals` + fit report display,
   "Generate" button gated on fit-gate pass, resume version display once
   generated, "Download .docx" button once rendered
4. TanStack Query hooks for each new endpoint, following existing
   query/mutation conventions in the codebase
5. Go: `POST /resume-versions/{id}/render` handler calls the Python renderer
   over HTTP (base URL from a new `RENDERER_URL` env var), streams DOCX
   bytes back with correct `Content-Type` and `Content-Disposition` headers
   for a browser download
6. Error states: fit-gate hard fail (show gaps clearly, no generate button),
   generation failure, renderer unavailable

## Explicitly deferred

- Contributions editor (separate, equal-priority session — scope next)
- Batch/bulk operations of any kind
- Resume version history/diffing beyond a simple list

## Session conventions

- `git pull` and `gh issue list --state open` before starting, per standing
  convention
- Add `RENDERER_URL` to `.env.example` alongside `DATABASE_URL`
- Pair this session with a new GitHub issue before starting:

```
gh issue create \
  --title "Application generation flow: JD paste -> fit -> generate -> render" \
  --body "Minimal frontend flow to run a JD end-to-end: paste JD, Stage 1 signals + fit report, Stage 2 generate, render/download via Python renderer. See cc_sessions/NNN_application_generation_flow.md for full scope." \
  --label "frontend,stage-2,renderer"
```
