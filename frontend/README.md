# Role Model — Frontend

React + TypeScript client for [Role Model](../README.md), built on Vite. Talks to the Go API over REST.

## Status

The auth shell — login, signup, session persistence, and 401-driven redirect handling — plus the application generation flow: create an application from a pasted job description, extract JD signals, review the fit assessment, generate a resume, and download the `.docx`.

Vitest covers login, signup, session storage, session expiry, the API client, the error boundary, and `ApplicationDetail` (where the signal/fit/generate/render steps live). `Applications` and `ApplicationNew` have no tests of their own yet.

Career-data views (browsing and editing employers, positions, and contributions) don't exist yet. Career history is seeded rather than written through the UI — see the [root README](../README.md) for why — so those views are for reading and curating, not primary data entry.

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

Requires `VITE_API_BASE_URL` pointing at a running instance of the backend (see the [root README](../README.md) for backend setup). Copy the example file in this directory and adjust it:

```bash
cp .env.example .env
```

```
VITE_API_BASE_URL=http://localhost:8080/api/v1
```

**Include the `/api/v1` path.** The API client concatenates endpoint paths onto this value verbatim (`${API_BASE_URL}/applications`) and adds no prefix of its own, so a bare origin produces a 404 on every request.

The backend's `CORS_ALLOWED_ORIGINS` must include this dev server's origin. From the repository root, `make run-frontend` starts the dev server with `--host` so it is reachable from other machines on the network.

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

- `src/routes` — page-level components (`Login`, `Signup`, `Applications`, `ApplicationNew`, `ApplicationDetail`)
- `src/components` — shared components (`RequireAuth`, the auth-gated route wrapper; `ErrorBoundary`)
- `src/contexts` — React context providers (`AuthContext`)
- `src/hooks` — TanStack Query hooks (e.g. `useApplications`)
- `src/lib` — API client, session storage helpers, and shared types
- `src/test` — cross-cutting test setup and integration-style tests (e.g. session expiry)
