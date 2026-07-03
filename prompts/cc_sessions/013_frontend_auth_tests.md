Project: Role Model frontend — React + TypeScript + Vite.
Repository: https://github.com/shurikai/role-model

Add a test setup (none exists today) and write tests for the auth shell
built in 009_frontend_auth_shell.md. This is scoped narrowly: the four
files below, chosen because they're where real bugs have already surfaced
(the session-expiry redirect race) or where a silent regression would be
expensive to notice (api-client's 401 handling, which every future feature
depends on). This is not a coverage-percentage exercise — do not add tests
for components with no meaningful branching (e.g. don't test that a div
renders).

## Tooling
- vitest, @testing-library/react, @testing-library/jest-dom,
  @testing-library/user-event, jsdom — add as devDependencies, let npm
  resolve current versions
- frontend/vite.config.ts: add a `test` block — `environment: 'jsdom'`,
  `setupFiles: ['./src/test/setup.ts']`. Do NOT set `globals: true` — this
  codebase has no ambient-global precedent anywhere (every test file
  imports `describe`/`it`/`expect`/`vi` explicitly from `'vitest'`,
  matching the explicit-import style already used throughout)
- frontend/src/test/setup.ts: import `'@testing-library/jest-dom/vitest'`
  for matchers; `afterEach(() => localStorage.clear())` so tests don't
  leak session state into each other
- frontend/package.json: add `"test": "vitest run"` and
  `"test:watch": "vitest"` scripts

## Test files and required assertions

### frontend/src/lib/session.test.ts
- `getSession()` returns `null` when localStorage is empty
- `setSession(...)` followed by `getSession()` round-trips the exact
  object
- `clearSession()` removes the key — subsequent `getSession()` is `null`
- `getSession()` returns `null` (does not throw) when the stored value is
  malformed JSON — write garbage directly via
  `localStorage.setItem('role_model_session', 'not json')` first

### frontend/src/lib/api-client.test.ts
Mock global `fetch` with `vi.fn()` per test (restore after each).
- When a session exists (seed via `setSession`), the request is sent with
  an `Authorization: Bearer <token>` header
- When no session exists, no `Authorization` header is sent
- `Content-Type: application/json` is set only when `init.body` is
  present, not on bodyless GETs
- On a non-2xx JSON error body `{error, code}`, the thrown `ApiError` has
  `.message`, `.status`, `.code` matching the body exactly
- On a non-2xx response with a non-JSON body (mock `.json()` to reject),
  it still throws `ApiError`, falling back to the generic
  `Request failed with status ${status}` message and `code: "unknown_error"`
  — must not throw an unrelated parse error instead
- **Regression test for the race CC already fixed by hand:** register a
  handler via `setUnauthorizedHandler(fn)`, mock a 401 response, call
  `apiFetch`, and assert the handler ran — specifically assert it was
  called before the `apiFetch` promise's rejection was observed by the
  caller (e.g. track call order with a shared array both the handler and
  the `.catch()` push into, assert the handler's entry comes first).
  This is the actual behavior AuthContext depends on; encode it directly
  rather than only testing it indirectly through the component test below.
- On a 204 response, `apiFetch` resolves `undefined` and never calls
  `.json()` on the response (assert the mock's `json` method was not
  invoked)

### frontend/src/test/session-expiry.test.tsx
Component-level regression test for the exact bug found in 009: render
`AuthProvider` + a `RequireAuth`-protected route inside a `MemoryRouter`
(seed a valid session first so it starts authenticated and the protected
route renders). Trigger a 401 the same way production code would — mock
`apiFetch`'s underlying `fetch` to return 401 for some in-flight call, or
invoke the registered unauthorized handler directly if that's more
stable than depending on a specific request happening to fire.
Assert the end state: the router lands on `/login` with BOTH `redirect`
and `reason=expired` present in the query string, and that this holds
whether or not `AuthContext`'s own effect also runs — i.e. the test
should fail if a duplicate/competing navigation reintroduces the
dropped-param bug. Read AuthContext.tsx and RequireAuth.tsx first to
match the actual current implementation, not the original prompt's spec
(they diverged for a documented reason — see the code comments in both
files).

### frontend/src/routes/Login.test.tsx
Render `Login` inside a `MemoryRouter` + `AuthProvider`, mock `apiFetch`'s
underlying `fetch`.
- Submitting the form with values typed into the email/password inputs
  calls the login flow with exactly those values
- On an `ApiError` with `code: "invalid_credentials"`, the message shown
  is the verbatim backend text ("invalid email or password")
- On an `ApiError` with `code: "internal_error"`, the message shown is
  the generic fallback, NOT the raw backend message. Check the current
  source of Login.tsx/Signup.tsx before writing this — if the
  internal_error-to-generic mapping isn't actually implemented yet, this
  test will fail; in that case implement the mapping (a small
  `code === "internal_error" ? "Something went wrong. Please try again."
  : err.message` conditional in the catch block) rather than skipping
  the test
- Visiting `/login?reason=expired` renders the expiry banner; clicking
  its dismiss button hides it

### frontend/src/routes/Signup.test.tsx
Same structure as Login.test.tsx, scoped to whatever branching Signup.tsx
actually has (it has no expiry-banner logic, so that assertion doesn't
apply here — don't invent a parallel test that doesn't map to real code).

## Constraints
- Every test file uses explicit imports from `'vitest'`
  (`describe, it, expect, vi, beforeEach, afterEach`) — no globals
- Do not add tests for `App.tsx`, `main.tsx`, or `types.ts` — no
  meaningful behavior to assert there
- Do not chase a coverage number; if you find yourself testing that a
  label renders next to its input, stop
- `npm run test` must pass, `npm run build` must still pass, before
  finishing
