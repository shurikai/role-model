# Phase 1 scope: career-data UI (#118) + minimal shell (#143)

**Status:** proposed, 2026-10-23. Not yet started.
**Goal:** a non-technical tester can see, correct and hide what any of the
three on-ramps (career import, add contributions, career interview) wrote to
their record, and can find their way around without knowing the data model.
**Out of scope:** OIDC (#120), the full information-hierarchy pass of #143,
tag editing on contributions, projects UI, education/credentials UI.

Ships as three PRs, in order. Each is independently mergeable and leaves the
app better than it found it.

---

## Why this order

#143 says its job is the shell that #118's screens slot into. Building #118
first means restyling it again. But a full #143 pass is a design project, and
the gate only needs navigation a stranger can follow. So: a thin shell first
(PR A), then read-only career views inside it (PR B), then editing (PR C).
PR B alone closes the gap called out on #118 (no surface at all after an
onboarding interview).

---

## What exists today (verified against main @ 9ed4fc5)

**Frontend**
- `AppShell` has five flat links: Applications, Profile, Career import, Add
  contributions, Career interview. Three of the five are on-ramps that do the
  same job by different means.
- Every route owns its own column (`mx-auto max-w-{3,4,5}xl px-6 py-10`) and
  repeats the mono eyebrow + heading pattern by hand.
- `hooks/useCareer.ts`: `useEmployers`, `useEmployerPositions`,
  `useCreateEmployer`, `useCreatePosition`. No contribution hooks, no update or
  delete hooks.
- `/` redirects to `/applications`, which is useless to a new account with no
  career data.

**Backend**
- Full CRUD routes for employers, positions, contributions
  (`internal/api/router.go:80-98`).
- **Updates are full replacement.** `updatePositionRequest` includes
  `started_on`, `sort_order` and every nullable field; `updateContributionRequest`
  requires `summary` and `full_description`. The UI must round-trip the whole
  object or it will null out fields the form didn't show.
- **Delete semantics differ by entity:**
  - Employer `DELETE` → 409 `has_dependents` if it has positions.
  - Position `DELETE` → 409 `has_dependents` if it has contributions.
  - Contribution `DELETE` is a **hard** delete (tags, project links and
    feedback removed in one tx, `internal/contribution/service.go`).
  - Only contributions have `is_active`. Generation already skips inactive
    contributions (`internal/generation/assemble.go:63`).
- **No way to read a contribution's tags over HTTP.** `GetTagsByContribution`
  exists in `tags.sql`, but the only `/contributions/{id}/tags` routes are POST
  and DELETE.
- **No way to list import batches.** `ListImportBatches` exists in
  `import_batches.sql` with no route, so an abandoned review is reachable only
  by its URL.
- No endpoint returns a user's whole career. Browsing with today's API is a
  waterfall: employers → positions per employer → contributions per position.

---

## PR A: shell and navigation (#143, minimal)

### Navigation model

Replace the five flat links with three areas plus one action:

| Nav item | Route | Contains |
|---|---|---|
| **Career** | `/career` | Employers → positions → contributions (PR B/C) |
| **Applications** | `/applications` | Unchanged |
| **Skills & preferences** | `/profile` | Unchanged content; label only |
| **+ Add to career** (button, not a tab) | `/career/add` | Hub for the three on-ramps |

`/career/add` is a small page with three cards, written for someone who does
not know the internals:

1. **I have a resume or CV** → `/import/career/new`
2. **Add accomplishments to a job already here** → `/import/new`
   (disabled with a one-line explanation when the account has no positions)
3. **I'd rather talk it through** → `/onboarding/new`

Below the cards: **Unfinished imports**, a list of batches whose drafts are
not all resolved, each linking to its review screen. Needs the
`GET /import` endpoint below.

Existing routes keep their URLs. Only how you reach them changes.

### Root redirect

`/` → `/career`. An empty career shows the empty state (PR B), which points at
`/career/add`. Applications are meaningless before career data exists, so
leading there was backwards for a new user.

### Shared layout

- `components/Page.tsx`: `<Page width="narrow|default|wide" eyebrow title
  actions>`; owns the column width, padding, eyebrow/heading and an optional
  right-aligned action slot. Migrate every route to it. This is mechanical and
  should change no visible spacing except where pages currently disagree.
- `components/EmptyState.tsx` and `components/ConfirmDialog.tsx`, used by PR B/C.
- Keep the Kanagawa Lotus tokens; no visual redesign in this PR.

### Backend

- `GET /api/v1/import` → the user's batches, newest first, with
  `{id, kind, status, created_at, pending_count}`. `kind` distinguishes the
  narrow and career paths so the hub links to the right review route.
  `import_batches` has no such column today; derive it in the query
  (`career` when the batch has any `entity_drafts` rows, else `contributions`)
  rather than adding a migration. Compute `pending_count` in the same query,
  not client-side.

### Tests

- `AppShell.test.tsx`: update for the new links; assert the active state on
  `/career/positions/:id` keeps **Career** highlighted (nested routes: the
  current `end` on every link will break this, so Career needs prefix matching).
- `CareerAdd.test.tsx`: three cards render; card 2 disabled with no positions;
  unfinished imports listed and linked by kind.
- Handler test for `GET /import`, user-scoped.

### Done when

- No on-ramp is a top-level nav item, and every existing screen is reachable
  from the nav in at most two clicks.
- Every route renders inside `Page`.
- `make test-all` green.

---

## PR B: read-only career view (#118, part 1)

### Backend: one career read endpoint

`GET /api/v1/career` returns the whole tree in one response:

```jsonc
{
  "employers": [
    {
      "id": "…", "name": "…", "industry": "…", "notes": "…",
      "positions": [
        {
          "id": "…", "title": "…", "started_on": "2022-02-01", "ended_on": null,
          "industry_level": "…", "industry_role": "…", "location": "…",
          "contribution_counts": { "active": 9, "hidden": 1 }
        }
      ]
    }
  ]
}
```

- Three sqlc queries (employers, positions by user, contribution counts by
  position grouped on `is_active`), assembled in Go. No ORM, no
  `database/sql`, per CLAUDE.md.
- Order employers by most recent position end date (current first), positions
  by `sort_order` then `started_on DESC`. Resume order, not alphabetical: it's
  how people think about their own history.
- Contributions are **not** inlined: a long career is hundreds of them. They
  load per position.

`GET /api/v1/positions/{id}/contributions` gains each contribution's tags
(`[{id, name, category}]`) via a single `ListContributionTagsByPosition` query,
so the position page is two requests, not N+1. Additive field; existing
callers ignore it.

### Frontend routes

| Route | Shows |
|---|---|
| `/career` | Employers as sections, each listing its positions (title, dates, location, contribution count). Empty state when there are none, pointing at **+ Add to career**. |
| `/career/positions/:id` | Breadcrumb (Career / Employer / Position), position fields, contributions as cards: summary visible, full description / outcomes / scale context expandable, tags as chips. Hidden contributions collapsed under "Hidden from resumes (n)". |

No employer detail page in the MVP. An employer is a header on `/career` and a
breadcrumb segment; it has too little content to earn a screen.

### Completion links

Close the loop the #118 comment describes:
- Onboarding interview finishes → "See what was added" → `/career`.
- Import review: when the last draft in a batch is resolved → same link.

### Tests

- Go: handler + integration test for `GET /career` (empty user, ordering,
  counts split by `is_active`, user isolation).
- Vitest: `/career` empty, populated, ordering preserved; position page renders
  tags, expands detail, groups hidden contributions.

### Done when

After an onboarding interview or a career import, a tester can see every
employer, position and contribution that was written, including tags, without
leaving the browser.

---

## PR C: editing (#118, part 2)

### Behaviour

- **Employer:** edit name / industry / notes inline on `/career`. Delete
  offered only when it has no positions; on 409 show the server's message.
- **Position:** edit form on the position page (all fields in
  `updatePositionRequest`). Delete offered only when it has no contributions.
- **Contribution:** edit in place on its card (summary, full description,
  outcomes, scale context).
  - **Primary removal is "Hide from resumes"** (`is_active = false`), reversible,
    one click, no confirm. This is what a tester actually wants when the
    interview or import wrote something wrong-ish.
  - **Delete** is secondary, behind `ConfirmDialog`, and the dialog says it is
    permanent.
- **Add:** "Add position" on an employer, "Add contribution" on a position,
  using the same forms. (Employer creation already exists in `PositionPicker`;
  reuse, don't duplicate.)

### Rules

- Forms submit the **full** object (see "Updates are full replacement" above).
  Build the PATCH body from the fetched entity merged with the form, never from
  the form alone. Add a test that a field not shown in the form survives a save.
- Mutations invalidate `["career"]` and the affected
  `["positions", id, "contributions"]` key.
- Dates: the form takes `YYYY-MM` and sends `YYYY-MM-01`, matching how Stage 0
  writes them. Show "Present" for a null `ended_on`.
- Tags stay read-only in this PR.

### Tests

- Vitest per form: edit round-trip preserves untouched fields; hide/unhide;
  delete confirm; 409 surfaced on employer/position delete.
- No new backend endpoints expected. If one turns out to be needed, stop and
  record why in the PR.

### Done when

The #118 acceptance criteria are met: list + detail for employers, positions
and contributions; edit and soft-delete from the UI; inside the shared shell;
Vitest coverage in line with the rest of `frontend/`.

---

## Decisions (approved 2026-10-23)

1. **GET /career tree endpoint vs. composing existing endpoints.** Decided: the tree endpoint. Composing needs 1 + E requests before the page can render and still can't show contribution counts. The cost is one handler and three queries.
2. **Contribution removal:** hide-first vs. delete-first. Decided: hide-first, as above. Hard delete stays available for junk.
3. **Rename "Profile" to "Skills & preferences".** Decided: rename, since "Profile" suggests name and contact details to a non-technical user. Label only; the route stays /profile.
4. **Tag editing on contributions.** Decided: out of MVP. Revisit after testers have used read-only chips.

## Follow-ups this surfaces (file as issues)

- Onboarding agent writes directly to real tables with no review step (#118
  comment, 2026-09-13). With PR C, hide/edit after the fact is the review. Worth
  a sentence on #117 that this is the intended answer for the MVP.
- #143 remainder: information hierarchy on Applications and the import review
  screens, once testers have been through them.
