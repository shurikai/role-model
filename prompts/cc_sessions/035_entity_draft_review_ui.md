# CC Session: Entity Draft Review UI (Intake, Phase 8/9 Frontend)

## Context

`internal/intake` (extraction, collision detection, dependency-ordered
resolution) is real and tested, and `cmd/intakerun` proves the whole path
end to end for a brand-new account — but per its own doc comment it is
"deliberately not wired into the server." The only HTTP surface today is
`GET /import/{batchID}/entities` (list) and `POST /import/{batchID}/resolve`
(approve everything pending, all at once). There is no way to create a
batch and trigger extraction over HTTP, no way to reject or edit a single
drafted entity, and no frontend for any of it. `routes/` has `Login`,
`Signup`, `Applications`, `ApplicationDetail`, `ApplicationNew` — nothing
for import batches or entity drafts. A new signup lands straight on the
applications list with no path to get their career into the system at all.

This is the actual Phase 8/9 blocker: "a non-engineer cannot use the import
without first creating employers and positions by hand" (CLAUDE.md).

Good news found during discovery — **this is smaller than it looks**:

- `entity_drafts.status` already has a `rejected` state in its CHECK
  constraint (migration 024), unused until now.
- `MarkEntityDraftRejected` **already exists** as a sqlc query
  (`internal/db/queries/entity_drafts.sql`) — nothing calls it yet.
- `topoOrder` in `internal/intake/resolve.go` **already treats rejected
  drafts as excluded** and reports their dependents as unresolved with a
  clear reason ("depends on drafts that were not resolved: ..."). Rejecting
  a draft with dependents already fails safely today — no new resolver
  logic needed for that case.
- The narrow `ImportHandler` (contribution-only path, existing employers/
  positions) already has a working `Reject`, `Approve`, and `UpdateDraft`
  on `/import/drafts/{draftID}` — **mirror that handler shape** for the
  wide `IntakeHandler` rather than inventing a new pattern. Read
  `internal/api/handlers/import.go`'s `UpdateDraft` (partial update via
  `updatableDraftFields` allowlist), `Approve` (error-to-status-code
  switch), and `Reject` before starting Task 1.

Design decisions confirmed in conversation (not open for reinterpretation
mid-session):

1. **Reject and edit are per-draft, not batch-level.** The existing
   `ResolveBatch` (approve-everything-pending) stays as is — this session
   adds the missing per-item operations alongside it, it does not replace
   the batch resolve flow.
2. **Edit is full-payload-replace, not field-level PATCH**, unlike the
   narrow `ImportHandler.UpdateDraft`'s partial-update allowlist. The five
   entity kinds have five different payload shapes (see
   `internal/intake/resolve.go`'s `employerPayload` / `positionPayload` /
   `contributionPayload` / `skillPayload` / `preferencePayload`); the
   frontend form for each kind will always submit a complete object for
   that kind, so full-replace is simpler here and there's no allowlist to
   maintain per kind.
3. **Reject and edit are only legal on `pending` drafts.** Attempting
   either on an `approved` or `rejected` draft is a 409, not a silent
   no-op — mirror `stage0.ErrDraftNotPending` / the `Approve` handler's
   `errors.Is` switch pattern.
4. **The batch-create + extraction-trigger endpoint is in scope** — this
   session is the first time a browser can start an import at all, not
   just review one already staged via `cmd/intakerun`.
4a. **Per-draft approve is in scope, alongside reject and edit.**
   `ResolveBatch`'s own doc comment confirms incremental, multi-pass
   resolution is already how the resolver is designed to work ("drafts
   resolved by an earlier pass are already parents for this one") — a
   single-draft approve endpoint uses that same path, it isn't fighting
   the design. The one rule to enforce explicitly: approving a draft
   whose `depends_on` isn't fully resolved yet is a 409 ("approve the
   parent employer draft first"), never a silent auto-cascade of
   unreviewed parents. This mirrors the explicit
   reject-with-dependents-confirm behavior in decision 7 below — every
   crossing of a dependency boundary is explicit, not automatic, in
   either direction.
4b. **Route namespacing: this session's routes must not collide with
   session 034's.** 034 (already written, separately staged) builds the
   Kanagawa Lotus UI for the narrow, existing path — `ContributionDraft`s
   against employers/positions that already exist. This session builds
   the wide, whole-career-from-scratch path. Both are "import review"
   screens and could plausibly reach for the same route name if not
   deliberately kept apart. This session's frontend routes use an
   explicit `/import/career/...` prefix (not bare `/import/...`) to stay
   clear of whatever 034 claims — confirm against 034's actual routes
   before finalizing if both have landed by the time this session runs.
5. **Frontend nesting is capped at employer → position**, per the approved
   mockup. Contributions render as a collapsed count on their position
   card ("4 contributions drafted") that expands — not a third nesting
   level. Skills and preferences render as flat lists after the
   employer/position tree, each in their own labeled group.
6. **Flagged drafts get a visible reason line, not just a badge.**
   `DraftFlags.PreferenceCollisions` and `NewCategories` are advisory —
   render the actual flag content as text next to the draft, not just a
   warning icon with no explanation.
7. **Rejecting a draft with dependents requires a confirm step** naming
   what will be orphaned ("2 positions and 6 contributions depend on
   this"), computed client-side from the already-fetched draft list's
   `depends_on` edges — no new backend endpoint needed for this, it's a
   graph walk over data the frontend already has.

Stack: Go (`internal/api/handlers/intake.go`, `internal/api/router.go`,
`internal/db/queries/entity_drafts.sql`), sqlc regeneration, React/
TypeScript/Vite (new route + hook file), TanStack Query (mirror
`frontend/src/hooks/useApplications.ts`'s query/mutation pattern exactly —
`useQuery`/`useMutation`/`queryClient.invalidateQueries` on
`["import", batchID, "entities"]` as the query key), Tailwind (no Kanagawa
Lotus tokens exist in this repo yet — confirmed during discovery; this
session ships with existing/base Tailwind styling, same "no design pass"
standard as session 033's frontend task, not the Stage 0c Kanagawa Lotus
work).

---

## Session Start (always)

```bash
git pull
gh issue list --state open
```

Read `internal/api/handlers/import.go` in full (not just `UpdateDraft`/
`Approve`/`Reject`) before Task 1 — the wide `IntakeHandler` needs to feel
like the same codebase as the narrow one, not a reinvention.

Before Task 4, check `frontend/tailwind.config.*` for Kanagawa Lotus theme
tokens (session 034's deliverable). If they exist, use them for this
session's new screens. If they don't, use base Tailwind only — styling
stays out of scope either way, per the stack note above; this check just
avoids shipping throwaway styles if 034 happened to land first.

---

## Task 1: Backend — reject and edit endpoints on `IntakeHandler`

In `internal/db/queries/entity_drafts.sql`, add:

```sql
-- name: UpdateEntityDraftPayload :one
UPDATE entity_drafts
SET payload = $3, updated_at = now()
WHERE id = $1 AND user_id = $2 AND status = 'pending'
RETURNING *;
```

Note the `status = 'pending'` guard is in the query itself, not just
checked in Go first — this closes the race where a resolve happens between
the check and the update. `MarkEntityDraftRejected` needs the same guard
added (`AND status = 'pending'`) — it currently has none, which means
today it would happily "reject" an already-approved draft. Fix this as
part of this task, not as a drive-by — note in the commit message that
this is a pre-existing gap being closed, not new session-035 behavior.

Regenerate sqlc. In `internal/api/handlers/intake.go`, add:

- `RejectEntityDraft` (`POST /import/entities/{draftID}/reject`) — calls
  the now-guarded `MarkEntityDraftRejected`. A rows-affected-zero result
  (query matched nothing because the draft wasn't pending, wasn't the
  user's, or didn't exist) needs to distinguish "not found" from "not
  pending" for the response — check `GetEntityDraft` first if the query
  itself doesn't distinguish these via `pgx.ErrNoRows` vs. a business
  error, same pattern as `ImportHandler.Approve`'s switch.
- `UpdateEntityDraftPayload` (`PUT /import/entities/{draftID}`) — decode
  the full payload JSON for the draft's `Kind` (validate it round-trips
  through the matching payload struct from `internal/intake/resolve.go`
  before writing — an edit that produces an unparseable payload should
  fail at edit time with a 422, not surface as a mysterious resolve
  failure later), call `UpdateEntityDraftPayload`.
- `ApproveEntityDraft` (`POST /import/entities/{draftID}/approve`) — a
  single-draft version of what `ResolveBatch` does for the whole batch.
  Add a new `Service.ApproveDraft(ctx, userID, draftID uuid.UUID)
  (uuid.UUID, error)` in `internal/intake/resolve.go`, reusing
  `resolveOne` and `MarkEntityDraftResolved` inside its own transaction
  (mirror `ResolveBatch`'s `tx`/`qtx` setup). Before calling `resolveOne`,
  check every id in the draft's `depends_on` is either already resolved
  (has a `resolved_id`) — build this the same way `ResolveBatch` seeds its
  `result.Resolved` map, by loading the batch's other drafts. Return a
  distinct sentinel (`ErrDependencyNotResolved`) if any dependency isn't
  resolved yet, and have the handler map that to 409 with a message
  naming which dependency is missing — never silently resolve the
  dependency first. This is the approve-side mirror of decision 7's
  reject-side dependent-confirm: dependency boundaries are always crossed
  explicitly, never automatically, in either direction.

Register all three in `internal/api/router.go` next to the existing
intake routes:

```go
r.Post("/import/entities/{draftID}/approve", intakeHandler.ApproveEntityDraft)
r.Post("/import/entities/{draftID}/reject", intakeHandler.RejectEntityDraft)
r.Put("/import/entities/{draftID}", intakeHandler.UpdateEntityDraftPayload)
```

## Task 2: Backend — batch-create + extraction-trigger endpoint

There is currently no way to create an `import_batches` row and call
`intake.Service.ExtractCareer` over HTTP — only `cmd/intakerun` does this,
against a bare/new account. Add:

```go
r.Post("/import/career", intakeHandler.StartCareerImport)
```

`StartCareerImport` — accepts `{ "raw_text": string }`, creates the
`import_batches` row (status `extracting`), calls `ExtractCareer` the same
way `cmd/intakerun`'s `main.go` does, returns the batch id and draft
counts (mirror `ImportHandler.Create`'s `createImportBatchResponse` shape
for consistency, but this is a distinct endpoint — do not attempt to
unify the narrow and wide import-creation paths in this session, that's a
bigger refactor than this scope). This endpoint assumes an existing
authenticated user (from signup) — it does **not** replicate
`cmd/intakerun`'s user-creation step; a session that already reached the
authenticated app has a user.

Confirm whether `vocabulary.Install` (neutral default vocab) already runs
at signup — check `internal/api/handlers/auth.go`'s signup handler. If it
doesn't, that's a separate, real gap (a signed-up user with no vocabulary
rows would break skill/tag resolution during extraction) — flag it in the
session notes rather than silently fixing it inline, since it may be
already handled by a DB default or trigger this session hasn't looked at
yet.

## Task 3: Frontend — hooks

New `frontend/src/hooks/useIntake.ts`, mirroring `useApplications.ts`'s
pattern exactly:

```ts
useStartCareerImport()   // mutation, POST /import/career
useEntityDrafts(batchId) // query, GET /import/{batchID}/entities
useResolveBatch(batchId) // mutation, POST /import/{batchID}/resolve
useApproveEntityDraft()  // mutation, POST /import/entities/{id}/approve
                          // — surface ErrDependencyNotResolved's 409
                          // distinctly in the UI (Task 4): the card's
                          // approve action should show which dependency
                          // is still pending, not a generic error.
useRejectEntityDraft()   // mutation, POST /import/entities/{id}/reject
useUpdateEntityDraft()   // mutation, PUT /import/entities/{id}
```

All mutations invalidate `["import", batchId, "entities"]` on success,
matching the invalidation pattern in `useApplications.ts`.

## Task 4: Frontend — routes

- `ImportStart.tsx` (`/import/new` or similar) — text area, one submit
  button calling `useStartCareerImport`, navigates to the review route on
  success. Loading state matters — this is a real LLM call against a
  whole career, not instant; disable the button and show a distinct
  "extracting..." state, not just a spinner indistinguishable from a
  normal page load.
- `EntityDraftReview.tsx` (`/import/career/{batchId}`, per decision 4b's
  namespacing) — the approved mockup: employers as top-level cards,
  positions nested one level under their employer (via `depends_on`, not
  assumed order), contributions collapsed to an expandable count per
  position, skills and preferences as flat grouped lists after. Each
  draft card: approve/reject/edit actions, flagged drafts get a
  warning-styled border and the flag reason rendered as visible text
  (not hover-only). Approve on a draft with an unresolved dependency
  (the `ApproveEntityDraft` 409 case) shows the specific missing
  dependency inline on the card rather than a generic toast. Reject on a
  draft with dependents (computed client-side by scanning other drafts'
  `depends_on` for this draft's id) shows a confirm dialog naming what
  depends on it before calling the mutation.
- Wire `Signup.tsx`'s post-signup redirect (or `Applications.tsx`'s empty
  state) toward `/import/new` for a user with zero employers — check
  whether an existing "does this user have any career data yet" signal
  exists to gate this, or whether that's a fast-follow rather than this
  session's scope; don't invent a new backend check for it if nothing
  clean exists today, just route new signups to `/import/new` directly
  and leave existing users' navigation untouched.

## Task 5: Tests

- Go: table tests for `RejectEntityDraft` and `UpdateEntityDraftPayload`
  covering the pending-only guard (reject/edit an approved draft → 409),
  not-found (wrong id or wrong user → 404), and the happy path. A test
  confirming `MarkEntityDraftRejected`'s newly-added guard actually
  blocks a reject on a non-pending draft — this is the regression test
  for the pre-existing gap closed in Task 1.
- Go: `StartCareerImport` — at minimum a test that a batch gets created
  and `ExtractCareer` is invoked; mirror how `cmd/intakerun`'s behavior is
  (or isn't) covered today for the shape of a reasonable test double
  around the Anthropic call.
- Frontend: `EntityDraftReview.test.tsx` — render with a fixture set of
  drafts including one flagged and one with a dependent, confirm the
  reject-with-dependents confirm dialog appears and names the dependents,
  confirm a flagged draft's reason text renders.

---

## Do NOT

- Do not attempt a Kanagawa Lotus styling pass in this session — no
  tokens exist in this repo yet; that is separate, later work. Use
  existing/base Tailwind only, matching session 033's frontend standard.
- Do not add a third nesting level for contributions under positions —
  collapsed count + expand, per the approved mockup.
- Do not touch `ResolveBatch` or its all-or-nothing batch-approve
  behavior — this session adds per-draft reject/edit alongside it, not a
  replacement.
- Do not make `UpdateEntityDraftPayload` a partial/field-level PATCH —
  full-payload-replace only, per design decision 2.
- Do not auto-cascade-approve an unresolved dependency when approving a
  draft that depends on it — fail explicitly with the named dependency,
  per decision 4a. This applies symmetrically to reject's dependent-
  confirm behavior in decision 7 — neither direction of the dependency
  edge resolves silently.
- Do not route this session's frontend under bare `/import/...` — use
  `/import/career/...` per decision 4b to stay clear of session 034's
  routes.
- Do not unify the narrow (`ImportHandler`, contribution-only) and wide
  (`IntakeHandler`, whole-career) import paths into one — they stay
  separate per the existing architecture; mirror the narrow handler's
  *shape*, don't merge the two.
- Do not replicate `cmd/intakerun`'s user-creation step in
  `StartCareerImport` — this endpoint assumes an authenticated user
  already exists.

---

## Verification Steps

```bash
go build ./...
go test ./internal/intake/... -v
go test ./internal/api/... -v
go test ./...
cd frontend && npm run build && npm run test
```

Session is complete when: a signed-up user can paste raw career text at
`/import/career/new`, land on a review screen showing every extracted
employer/position/contribution/skill/preference in dependency order,
approve a single draft (and get a clear 409 if its dependency isn't
resolved yet), reject a draft and see its dependents correctly reported
as unresolved (both individually and on batch resolve), edit a draft's
payload and see the edited value persist through resolution, see flagged
drafts' reasons as visible text, and successfully resolve a batch into
real rows — end to end, no manual DB seeding, no `cmd/intakerun` involved.
`go build ./...` and the frontend build are both clean, the pre-existing
missing-guard gap on `MarkEntityDraftRejected` is closed with a
regression test, and this session's routes don't collide with session
034's.
