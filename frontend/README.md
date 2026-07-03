# Role Model — Frontend

React + TypeScript client for [Role Model](../README.md), built on Vite. Talks to the Go API over REST.

## Status

Auth shell only, so far: login, signup, session persistence, and 401-driven redirect handling, with Vitest coverage. Views for employers/positions/contributions/applications don't exist yet — this will grow as the backend surface does.

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

Requires `VITE_API_BASE_URL` pointing at a running instance of the backend (see the [root README](../README.md) for backend setup). Configure it in a `.env` file in this directory, e.g.:

```
VITE_API_BASE_URL=http://localhost:8080
```

The backend's `CORS_ALLOWED_ORIGINS` must include this dev server's origin.

## Scripts

- `npm run dev` — start the Vite dev server
- `npm run build` — type-check (`tsc -b`) and build for production
- `npm run preview` — preview a production build locally
- `npm test` — run the Vitest suite once
- `npm run test:watch` — run Vitest in watch mode
- `npm run lint` — run Oxlint

## Structure

- `src/routes` — page-level components (`Login`, `Signup`, …)
- `src/components` — shared components (e.g. `RequireAuth`, the auth-gated route wrapper)
- `src/contexts` — React context providers (`AuthContext`)
- `src/lib` — API client and session storage helpers
- `src/test` — cross-cutting test setup and integration-style tests (e.g. session expiry)
