# Role Model — Frontend

React + TypeScript client for [Role Model](../README.md), built on Vite. Talks to the Go API over REST.

## Status

Ten routes, all inside a nav shell that also carries sign-out:

| Route                     | What it is                                                                                                      |
| ------------------------- | --------------------------------------------------------------------------------------------------------------- |
| `/login`, `/signup`       | auth, session persistence, 401-driven redirect                                                                  |
| `/applications`           | the list, with a pipeline-stage badge per row                                                                   |
| `/applications/new`       | paste a job description, create, extract signals                                                                |
| `/applications/:id`       | signals, fit report, generate, download the `.docx`                                                             |
| `/import/career/new`      | **the wide import** — paste a career, get a review queue                                                        |
| `/import/career/:batchID` | review employers, positions, contributions, skills, preferences, education and credentials as a dependency tree |
| `/import/new`             | the narrow import — contributions against positions that already exist                                          |
| `/import/:batchID`        | its review queue                                                                                                |

Vitest covers login, signup, session storage, session expiry, the API client,
the error boundary, the app shell, both review queues, `PositionPicker` and
`ApplicationDetail`. `Applications`, `ApplicationNew`, `ImportStart`,
`CareerImportStart`, `PayloadEditor` and the hooks have no tests of their own.

**Not here yet:** browsing or editing career data after import, and any UI for
skills and preferences. Both have a full API; the fit gate scores against
preferences, so that screen is the gap most worth closing next.

## Stack

- **Vite** + **React 19** + **TypeScript**
- **TanStack Query** for server state
- **React Router** for routing
- **Tailwind** (via `@tailwindcss/vite`) for styling
- **Vitest** for tests

## Getting started

```bash
npm install
npm run dev        # start the dev server (default: http://localhost:5173)
```

Requires `VITE_API_BASE_URL` pointing at a running backend (see the
[root README](../README.md)). Copy the example file in this directory:

```bash
cp .env.example .env
```

```
VITE_API_BASE_URL=http://localhost:8080/api/v1
```

**Include the `/api/v1` path.** The API client concatenates endpoint paths onto
this value verbatim (`${API_BASE_URL}/applications`) and adds no prefix of its
own, so a bare origin produces a 404 on every request.

**This value is inlined at build time**, not read at runtime — which is why the
container image serves the API from the same origin and builds with a relative
`/api/v1` instead. An absolute URL is correct for exactly one way of reaching
the instance and wrong for every other. Here in dev it is fine, because you
know where your own dev server is.

The backend's `CORS_ALLOWED_ORIGINS` must include this dev server's origin —
`http://localhost:5173` by default. That only applies on this path; the
containerised deployment is same-origin and needs no CORS at all. From the
repository root, `make run-frontend` starts the dev server with `--host` so it
is reachable from other machines, and `make dev` starts it alongside the API
and the renderer.

## Scripts

- `npm run dev` — start the Vite dev server
- `npm run build` — type-check (`tsc -b`) and build for production
- `npm run preview` — preview a production build locally
- `npm test` — run the Vitest suite once
- `npm run test:watch` — run Vitest in watch mode
- `npm run lint` — run Oxlint
- `npm run format` — format with Prettier
- `npm run format:check` — verify formatting without writing

## Structure

- `src/routes` — page-level components, one per route in the table above
- `src/components` — `AppShell` (nav and sign-out), `RequireAuth`,
  `AuthCard` and its form primitives, `PayloadEditor`, `PositionPicker`,
  `ErrorBoundary`
- `src/contexts` — React context providers (`AuthContext`)
- `src/hooks` — TanStack Query hooks (`useApplications`, `useCareer`,
  `useImport`, `useIntake`)
- `src/lib` — API client, session storage helpers, and shared types
- `src/test` — cross-cutting setup and integration-style tests (session expiry)

## Styling

Kanagawa Lotus, defined as `@theme` tokens in `src/index.css` — reach colours
as Tailwind utilities (`bg-paper`, `text-ink`, `border-verify`) rather than as
hex literals, so there is one place to change one. Fonts are self-hosted
through `@fontsource`. There are no rounded corners anywhere; the square,
document-like edge is deliberate.
