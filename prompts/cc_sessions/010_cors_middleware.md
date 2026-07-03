Project: Role Model — a self-hostable Go REST API for AI-powered resume generation.
Repository: https://github.com/shurikai/role-model

Add CORS middleware. The browser blocks every frontend->backend request today
(signup/login/me all fail with a CORS error in a real browser) because the
API sends no Access-Control-* headers. curl doesn't surface this since CORS
is a browser-enforced policy, not a server-side check — that's why this
wasn't caught until frontend browser testing.

## Why this needs to be config-driven, not a hardcoded localhost allowance
The frontend and backend will most likely be separate origins in production
too (frontend on Fly.io, possibly a different subdomain/app than the API),
so this isn't a dev-only convenience fix — hardcoding
`http://localhost:5173` would just mean re-discovering this exact bug at
deploy time. Allowed origins must come from config, matching every other
environment-dependent value in this codebase (DATABASE_URL, JWT_SECRET,
etc., all loaded via internal/config).

## End state

### go.mod
Add `github.com/go-chi/cors` — official chi-org CORS middleware, same
maintainers as go-chi/chi and go-chi/chi/middleware already in use, no new
unfamiliar dependency pattern.

### internal/config/config.go
Add to the Config struct:

    AllowedOrigins []string

In Load():
- Read `CORS_ALLOWED_ORIGINS` as a comma-separated list, trim whitespace on
  each entry, drop empty entries
- If unset AND Environment == "development": default to
  []string{"http://localhost:5173"} (Vite's default dev port)
- If unset AND Environment != "development": leave AllowedOrigins empty
  (fail closed — no cross-origin requests permitted until explicitly
  configured) and log a warning via the standard log package:
  "WARNING: CORS_ALLOWED_ORIGINS is not set; cross-origin requests will be
  rejected" — this must be loud enough to notice at deploy time, not a
  silent misconfiguration that looks like a working server until someone
  opens the frontend

### internal/api/router.go
- Add `AllowedOrigins []string` to `RouterDeps`
- Mount `cors.Handler` as the FIRST middleware in the chain, before
  `chimiddleware.Recoverer` and before the `RequireAuth` group — this
  matters because CORS preflight (OPTIONS) requests never carry the
  Authorization header, and cors.Handler needs to intercept and respond to
  them before they'd otherwise hit RequireAuth and get rejected as
  unauthenticated
- Options:
  - AllowedOrigins: deps.AllowedOrigins
  - AllowedMethods: {"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
  - AllowedHeaders: {"Authorization", "Content-Type"} — both are actually
    sent by the frontend's apiFetch wrapper (Bearer token + JSON body)
  - AllowCredentials: false — the frontend stores the JWT in localStorage
    and sends it via the Authorization header, not cookies, so there's no
    reason to enable credentialed CORS (which also forbids wildcard
    origins and adds complexity for no benefit here)
  - MaxAge: 300 (5 minutes — reasonable preflight cache, avoids a
    preflight round-trip on every single request without caching it so
    long that a config change takes forever to take effect)

### cmd/server/main.go
Pass `AllowedOrigins: cfg.AllowedOrigins` into the `api.RouterDeps{}`
struct literal alongside the existing fields.

### .env.example (or wherever DATABASE_URL/JWT_SECRET are documented today)
Add:
    CORS_ALLOWED_ORIGINS=http://localhost:5173

## Constraints
- Do not use AllowedOrigins: []string{"*"} anywhere, even as a fallback —
  wildcard origins are how CORS misconfigurations turn into real
  vulnerabilities, and it's not needed here since the origin list is
  config-driven
- Do not enable AllowCredentials — see rationale above, changing this
  later requires revisiting the origin-wildcard constraint too, so don't
  turn it on speculatively
- Run go build ./... and go vet ./... before finishing
- After this lands, the frontend session's login/signup/me calls from
  localhost:5173 should succeed with no code changes needed on the
  frontend side — this is purely a backend fix
