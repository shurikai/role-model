# CC Session: Stage 0c Review UI

## Context

Stage 0 (LLM-assisted import) is fully built on the backend — extraction,
enrichment, and the review/approve/reject endpoints all exist under
`/api/v1/import`. There is no frontend for any of it. Right now the only way
to get career data into the system is hand-written SQL seed files, which is
the single biggest blocker to Phase 1 ("usable by humans other than me").
This session builds the missing review screen.

A visual design pass happened separately (Claude web conversation, not this
repo) and produced a reference file at `design/reference/stage0c-kanagawa-lotus.jsx`.
That file is a **visual/interaction reference only** — it's a standalone
React mockup with inline styles and hardcoded hex values so it could render
in a preview sandbox, and it was built against an earlier, incorrect
assumption about the data model (a single original-vs-suggested text pair
per draft, plus a confidence score). Neither exists in the real schema. Pull
from it: the Kanagawa Lotus palette and elevation hierarchy, the type system
(Space Grotesk / Inter / IBM Plex Mono), the ledger-rail-with-colored-ticks
pattern, the left-edge-bar + filled-badge status treatment, and the general
restraint of the layout. Do NOT port its original-vs-suggested diff view or
confidence badge — those need to be redesigned against the real data below.

**Real per-draft shape** (`internal/db/models.go` `ContributionDraft`,
confirmed against `internal/stage0/service.go` and
`internal/generation/prompts/stage0b_enrichment.txt`):

```go
type ContributionDraft struct {
    ID              uuid.UUID
    UserID          uuid.UUID
    BatchID         uuid.UUID
    EmployerName    string
    PositionTitle   string
    Summary         *string
    FullDescription *string
    Outcomes        *string
    ScaleContext    *string
    Flags           *json.RawMessage // []{ type, field, message }
    Status          string           // pending / approved / rejected
    CreatedAt       time.Time
    UpdatedAt       time.Time
}
```

`Flags` is an array of `{type: "inference"|"gap"|"suggestion"|"warning",
field: "employer_name"|"position_title"|"summary"|"full_description"|
"outcomes"|"scale_context"|"general", message: string}`. There is no
confidence score anywhere in the pipeline — do not invent one. There is no
per-entry original text — the only raw source is the whole pasted blob at
the batch level (`GET /api/v1/import/{batchID}` returns the batch, not
per-draft originals). Do not build an original-vs-suggested comparison.

The editable content is four separate fields (`summary`, `full_description`,
`outcomes`, `scale_context`), not one blob. Each is independently editable
and independently flaggable.

**Approve requires a `position_id`** (`POST
/api/v1/import/drafts/{draftID}/approve`, body `{position_id}`). There is no
lookup-or-create-by-name — the position must already exist
(`stage0.ErrPositionNotFound` if not). Employer/position CRUD endpoints
exist (`GET /employers`, `GET /employers/{employerID}/positions`, `POST
/employers`, `POST /positions`) but there is no frontend for them at all.
Approving a draft therefore requires a position picker capable of creating
a new employer/position inline when the extracted `employer_name` /
`position_title` don't match anything existing — most imported drafts won't
match an existing position on a fresh account. Scope this session to make
that picker functional, not polished; a plain searchable select plus a
"create new" fallback is enough.

Stack: React 19 / TypeScript / Vite, TanStack Query, React Router, Tailwind
utility classes (no inline styles, no styled-components — check
`frontend/src/routes/Applications.tsx` for the existing convention). Tailwind
here is zero-config v4 (no `tailwind.config.*` file); custom colors need to
go through `frontend/src/index.css` `@theme` tokens, not arbitrary hex in
`className`.

---

## Session Start (always)

```bash
git pull
gh issue list --state open
```

---

## Task 1: Extend the Tailwind theme with the Kanagawa Lotus tokens

In `frontend/src/index.css`, add `@theme` custom properties for the palette
pulled from `design/reference/stage0c-kanagawa-lotus.jsx`'s `COLORS` object:
`paper`, `card`, `surface`, `ink`, `ink-dim`, `verify`, `flag`,
`flag-highlight`, `stamp`, `reject`, `rail`, `border`. Name them so they're
usable as `bg-paper`, `text-ink`, `border-verify`, etc. Also register the
three typefaces (Space Grotesk, Inter, IBM Plex Mono) — self-host via
`@fontsource` packages (`npm install @fontsource/space-grotesk
@fontsource/inter @fontsource/ibm-plex-mono`) rather than the Google Fonts
`@import` the mockup used; that was a sandbox-preview shortcut, not something
that belongs in a real app's network waterfall. Import the needed weights in
`main.tsx`.

## Task 2: Types

In `frontend/src/lib/types.ts`, add:

```typescript
export type DraftFlagType = "inference" | "gap" | "suggestion" | "warning";
export type DraftEditableField =
  | "employer_name" | "position_title" | "summary" | "full_description"
  | "outcomes" | "scale_context" | "general";

export interface DraftFlag {
  type: DraftFlagType;
  field: DraftEditableField;
  message: string;
}

export interface ContributionDraft {
  id: string;
  user_id: string;
  batch_id: string;
  employer_name: string;
  position_title: string;
  summary: string | null;
  full_description: string | null;
  outcomes: string | null;
  scale_context: string | null;
  flags: DraftFlag[] | null;
  status: "pending" | "approved" | "rejected";
  created_at: string;
  updated_at: string;
}

export interface ImportBatch {
  id: string;
  status: "pending" | "extracting" | "enriching" | "ready" | "failed";
  error_text: string | null;
  draft_count: number;
}
```

Confirm field names against actual JSON responses (`json:"..."` tags in
`internal/db/models.go` and `internal/api/handlers/import.go`) rather than
trusting this list blindly — write it, then grep the Go structs to check
you didn't miss a field or get a name wrong.

## Task 3: `useImport` hooks

New file `frontend/src/hooks/useImport.ts`, following the existing pattern
in `useApplications.ts` (TanStack Query, `apiFetch` from `../lib/api-client`):

- `useCreateImportBatch()` — mutation, `POST /import` with `{raw_text}`,
  returns `ImportBatch`.
- `useImportBatch(batchID)` — query, `GET /import/{batchID}`. Poll while
  `status` is `pending`/`extracting`/`enriching` (`refetchInterval`), stop
  once `ready` or `failed`.
- `useImportDrafts(batchID)` — query, `GET /import/{batchID}/drafts`,
  returns `ContributionDraft[]`.
- `useUpdateDraft()` — mutation, `PUT /import/drafts/{draftID}` with a
  partial body of only the four editable fields that changed (the handler
  rejects unknown keys — see `updatableDraftFields` in
  `internal/api/handlers/import.go` — so don't send `employer_name` or
  `position_title`, those aren't in the updatable set). Invalidate the
  drafts query on success.
- `useApproveDraft()` — mutation, `POST
  /import/drafts/{draftID}/approve` with `{position_id}`. Invalidate drafts
  query on success.
- `useRejectDraft()` — mutation, `POST /import/drafts/{draftID}/reject`, no
  body. Invalidate drafts query on success.

## Task 4: Position picker

New component `frontend/src/components/PositionPicker.tsx`. Needs:

- `useEmployers()` and `useEmployerPositions(employerID)` hooks (new, same
  pattern as Task 3) hitting `GET /employers` and `GET
  /employers/{employerID}/positions`.
- A searchable select over employers, then positions within the selected
  employer.
- A "none of these — create new" path: inline form for employer name (if
  creating new) and position title/dates, `POST /employers` then `POST
  /positions`.
- Pre-fill the search/create-new default from the draft's `employer_name` /
  `position_title` so the common case (no match, first-time import) is
  close to a two-click confirm rather than a blank form.
- Emits the resolved `position_id` to the parent via a callback prop. Keep
  this component free of Stage0c-specific styling assumptions — it'll get
  reused wherever else the app needs to attach something to a position.

## Task 5: `Stage0cReview` route

New route `frontend/src/routes/ImportReview.tsx`, mounted at
`/import/:batchID` in `App.tsx` (inside the `RequireAuth` group, same as
the other routes).

Layout, translated from `design/reference/stage0c-kanagawa-lotus.jsx` into
Tailwind classes against the Task 1 theme tokens — keep: the ledger rail
with per-outcome colored ticks, the left-edge status bar + filled status
badge with icon, the ID-cell / employer-title-stack header shape, the
ink-on-paper elevation stack. Replace:

- **No confidence badge** — nothing to show. In its place under the ID,
  show a flag-count summary instead: if `flags.length === 0`, a quiet
  "clean" indicator in `stamp` color; otherwise a count badge colored by
  the highest-severity flag type present on the draft (`warning` >
  `inference` > `gap` > `suggestion` — pick a color mapping across
  `flag`/`reject`/`rail` that reads as ascending severity, document the
  ordering in a comment since it's a judgment call, not derived from
  anything in the backend).
- **No original-vs-suggested diff.** Instead, four stacked editable
  sections per card — Summary, Full Description, Outcomes, Scale Context —
  each a labeled `<textarea>` (controlled, local state, `PUT` on blur or an
  explicit per-field save — pick whichever is less fiddly to wire
  correctly, don't overbuild an autosave debounce for this session) seeded
  from the draft's current value.
- **Flags render per-field**, directly under the relevant textarea, not as
  inline phrase-highlighting (there's no span-level location data, just a
  field name and a message). Group by field; a field with no flags shows
  nothing extra. Use the flag `type` for icon/color the same way status
  types got icons in the reference file — pick a reasonable icon per type
  (inference/gap/suggestion/warning) rather than reusing the check/x/pencil
  set that file used for review status, since these are a different
  vocabulary.
- **Approve** opens the `PositionPicker` (Task 4) rather than firing
  directly; only calls `useApproveDraft` once a `position_id` is resolved.
- **Reject** and **Skip-for-now** behave as in the reference file — Reject
  calls the mutation immediately, no body needed; Skip is local-only UI
  state (no backend call — there's no "skipped" status on the real
  schema, so skip just means "collapse and move on for this session,"
  it doesn't persist).
- Batch header (top of page): show the real batch status
  (`pending`/`extracting`/`enriching`/`ready`/`failed`) instead of the
  reference file's static "Imported from resume_2026.pdf" text, since
  that's real data now (`useImportBatch`). While status isn't `ready`,
  show a simple in-progress state instead of the draft list — there's
  nothing to review yet during extraction/enrichment.

## Task 6: Entry point

Somewhere reachable from `Applications.tsx` or a new lightweight
`ImportStart.tsx` route (`/import/new`) — a single textarea + submit that
calls `useCreateImportBatch()` and redirects to `/import/:batchID` on
success. Keep this minimal; it's the on-ramp, not a feature in itself.

---

## Do NOT

- Do not invent a confidence score, or backfill one from flag counts and
  call it "confidence" — it's a flag-count summary, name it and treat it
  as one.
- Do not build the original-vs-suggested diff view from the reference
  file. The data to support it doesn't exist; don't fake it by diffing
  the current field value against itself or against nothing.
- Do not let `PositionPicker` assume a match always exists — most drafts
  on a fresh import will need "create new." That path needs to actually
  work, not just be a stub button.
- Do not add inline styles or hex literals in the new components — every
  color must come through the Task 1 theme tokens. If a reference-file
  color doesn't have a token yet, add one in Task 1 rather than reaching
  for an arbitrary Tailwind value.
- Do not implement autosave-on-every-keystroke for the four text fields.
  Blur-triggered or explicit-save is enough for this session.
- Do not touch any backend code — everything needed already exists. If
  something in this prompt turns out to be wrong about the API shape,
  stop and flag it rather than patching the backend to match a frontend
  assumption.

---

## Verification Steps

```bash
cd frontend
npm run lint
npm run format:check
npm run build
npm run test
```

Session is complete when: a batch can be created from pasted text, the
review screen polls until extraction/enrichment finishes, all four fields
per draft are independently editable and persist via `PUT`, flags render
grouped by field with no phrase-highlighting, approving a draft resolves a
real `position_id` through a working create-new path, rejecting works, no
existing test regresses, and `npm run build` is clean.
