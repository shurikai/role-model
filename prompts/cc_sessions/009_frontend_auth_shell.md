Project: Role Model frontend — React + TypeScript + Vite, consuming the Go
REST API at internal/api (see api-contract.md for the full contract).
Repository: https://github.com/shurikai/role-model (backend only today —
this session creates the frontend from scratch in a new /frontend directory).

Scaffold the auth shell and API client only. No feature UI yet — no
dashboard, no applications table, no import flow. This session's job is:
a working login/signup screen, a persisted session, and a global 401
interceptor. Everything else is out of scope.

## Stack
- Vite + React + TypeScript
- React Router (latest stable) for routing
- TanStack Query for server state
- Tailwind for styling (minimal — this session has almost no visual surface)
- No component library, no state management library beyond React context +
  TanStack Query

## Prerequisite
`GET /api/v1/me` already exists on the backend (authenticated route,
returns `db.GetUserRow` via `db.Queries.GetUser`). Nothing to add there —
this session only consumes it from the frontend.

## API facts this session depends on (already confirmed against source —
do not re-derive or change these assumptions)
- Base URL: read from `VITE_API_BASE_URL` env var, no default baked in
- Auth: `POST /api/v1/auth/login` and `POST /api/v1/auth/signup`, body
  `{ email, password }`, response `{ token, user: { id, email } }` — this
  minimal shape is what gets persisted to `localStorage` for instant-paint
  rehydration; do not change it
- `GET /api/v1/me` (authenticated): returns the full `db.GetUserRow` —
  `{ id, email, full_name, phone, location, linkedin_url, github_url,
  site_url, headline, created_at, updated_at }`, all fields besides
  `id`/`email`/timestamps nullable. This session only uses it to verify
  the session is still valid and to refresh `id`/`email` in storage — the
  extra profile fields are unused until a future profile-page session,
  ignore them beyond typing the response correctly
- Token: JWT, 24h fixed expiry, **no refresh endpoint** — expiry is handled
  by catching 401, not by refreshing
- All other routes require header `Authorization: Bearer <token>`
- Every non-2xx response body is `{ error: string, code: string }` —
  build one error type around this, used everywhere

## End state

### frontend/src/lib/types.ts
Shared response types, starting with `AuthUser` (`{id, email}` — the
login/signup shape) and `GetUserRow` (the full `/me` shape described
above). Both `AuthContext.tsx` and the future profile-page session should
import from here rather than each defining their own inline shape.

### frontend/src/lib/session.ts
Typed wrapper around localStorage, single key `"role_model_session"` storing
`{ token: string, user: { id: string, email: string } }` as JSON.
Exports: `getSession()`, `setSession(session)`, `clearSession()`. No React
dependency in this file — pure storage helpers, easily testable.

### frontend/src/lib/api-client.ts
- `class ApiError extends Error { status: number; code: string }`
- `apiFetch<T>(path: string, init?: RequestInit): Promise<T>`:
  - Prepends `VITE_API_BASE_URL`
  - Attaches `Authorization: Bearer <token>` from `getSession()` if a
    session exists
  - Sets `Content-Type: application/json` when a body is present
  - On non-2xx: parse the `{error, code}` body (fall back to a generic
    message if the body isn't JSON), and:
    - if status is 401, call `handleUnauthorized()` (see below) BEFORE
      throwing, synchronously, so the redirect is already in flight by the
      time the caller's catch block runs
    - throw `new ApiError(message, status, code)` in all non-2xx cases
      (401 included) so TanStack Query's error state is always populated
  - On 2xx with an empty body (204), resolve `undefined` instead of trying
    to parse JSON
- `let unauthorizedHandler: (() => void) | null = null`
- `export function setUnauthorizedHandler(fn: () => void)` — called once by
  AuthProvider on mount to register itself; keeps api-client decoupled from
  React
- `function handleUnauthorized()` calls `unauthorizedHandler?.()` if set

### frontend/src/contexts/AuthContext.tsx
- `AuthProvider` holds `{ user: {id, email} | null, isAuthenticated: boolean }`
  in React state, initialized from `getSession()` on mount (synchronous
  read, no loading flicker for the common case) — this stays the
  instant-paint source of truth, `/me` never gates initial render
- Exposes `login(email, password)` and `signup(email, password)` — both call
  `apiFetch` against the respective endpoint, on success call `setSession`
  and update state, on failure let the `ApiError` propagate to the caller
  (the login form handles its own error display)
- Exposes `logout()` — calls `clearSession()`, clears state, navigates to
  `/login` (no API call — there's no logout endpoint, this is client-only)
- On mount, registers `handleSessionExpired` as the api-client's
  unauthorized handler via `setUnauthorizedHandler`:
  - `clearSession()`
  - clear user state
  - navigate to `/login?reason=expired&redirect=<current path>` (use
    `useNavigate` from react-router; current path from `useLocation`)
- **Background session verification:** once `isAuthenticated` is true, run
  `useQuery({ queryKey: ['me'], queryFn: () => apiFetch<GetUserRow>('/me'),
  enabled: isAuthenticated, retry: false, staleTime: Infinity })`. This is
  fire-and-forget from the UI's perspective:
  - Do NOT use its `isLoading` state to block or spinner-gate any route —
    the whole point is zero added latency on load. It runs after the
    already-rendered shell mounts.
  - On success: call `setSession` with the existing token plus the
    fresh `{id, email}` from the response (merge, don't drop the rest of
    the stored shape), so a same-tab email change elsewhere is picked up
    on next reload. Nothing else needs to happen — the point of this call
    for THIS session's scope is purely validation.
  - On error: do nothing extra here. If it was a 401, `apiFetch` already
    invoked `handleUnauthorized` before the query's `onError`/error state
    would even fire, so the redirect is already underway. If it was a
    network error (offline, backend down), do not log the user out —
    let the query just sit in an error state silently; there is no UI
    surface for this in this session's scope.
  - Only fire this query once per mount, not on window refocus — pass
    `refetchOnWindowFocus: false`. Refocus-triggered reverification is a
    reasonable future improvement but out of scope here (it would also
    need a decision about how aggressively to log people out of an idle
    tab, which isn't a call to make silently in this session).
- `useAuth()` hook throws if used outside the provider (standard guard)

### frontend/src/components/RequireAuth.tsx
Route guard component. If `!isAuthenticated`, `<Navigate to="/login" />`
preserving the attempted path as a `redirect` query param. Otherwise render
`<Outlet />`. Wrap all authenticated routes in this at the router level.

### frontend/src/routes/Login.tsx
- Email + password form, calls `useAuth().login`
- On success: navigate to the `redirect` query param if present, else `/`
- On mount: if `?reason=expired` is present, show a dismissible banner
  above the form: "Your session expired. Please log in again." — this is
  the ONLY UI surface for session expiry in this session's scope; there is
  no toast system yet, a simple conditional banner element is sufficient
- On submit failure: show `error.message` from the caught `ApiError`
  inline, do not use `alert()`

### frontend/src/routes/Signup.tsx
Same shape as Login, calling `useAuth().signup`, no `reason` banner needed.

### frontend/src/App.tsx / router setup
- `/login`, `/signup` — public
- `/` — wrapped in `RequireAuth`, renders a placeholder
  `<div>Authenticated shell — dashboard goes here</div>` (literally just
  that placeholder; the dashboard is a future session's scope)
- Wrap the whole app in `QueryClientProvider` (new `QueryClient` with
  default options: `retry: false` for now — 401s should never be retried,
  and we don't have a reason yet to retry anything else either) and
  `AuthProvider`

### frontend/.env.example
```
VITE_API_BASE_URL=http://localhost:8080/api/v1
```

## Constraints
- No component makes a raw `fetch()` call — everything goes through
  `apiFetch`
- No token or user data stored anywhere except via `session.ts`'s helpers
  (no scattered `localStorage.getItem` calls elsewhere)
- `api-client.ts` has zero imports from React or react-router — it must be
  usable outside a component tree (this is what makes the unauthorized-
  handler-registration pattern necessary instead of just calling
  `useNavigate` directly inside `apiFetch`)
- Do not implement retry-after-relogin / request queueing — out of scope,
  may come later specifically for the import flow
- Do not build a toast/notification system — the login page banner is
  sufficient for this session
- `npm run build` must succeed with zero TypeScript errors before finishing

## Open question to flag back to me, do not decide silently
`ApiError` includes `code` — should any specific `code` values (e.g.
`invalid_credentials`) get custom-worded messages on the login form instead
of the raw backend message, or is showing `error.message` verbatim
acceptable for now? Default to verbatim if you have to pick, but tell me
you picked it.
