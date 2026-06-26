Context: Role Model, Go service. Read CLAUDE.md. A previous refactor incorrectly
resolved an import cycle by DUPLICATING writeJSON/WriteError into two files:
handlers/respond.go and middleware/respond.go. This is wrong — there must be a
single shared implementation. Fix it properly.

## Target structure
Create a new package internal/httputil containing the single canonical copy of:
- WriteJSON(w http.ResponseWriter, status int, payload any)
- WriteError(w http.ResponseWriter, status int, code, message string)
- the errorResponse struct they use

Both the handlers and middleware packages must import internal/httputil and call
httputil.WriteJSON / httputil.WriteError. Neither handlers nor middleware may
import the other, and neither may define its own WriteJSON/WriteError.

## Also move the user-context accessor
The auth user-context plumbing currently lives in middleware (UserIDFromContext
and the context key). Move the context key and UserIDFromContext into
internal/httputil as well (or a dedicated internal/appctx package if you think
that's cleaner — if so, explain why before doing it). The goal: handlers can read
the authenticated user id WITHOUT importing middleware, and middleware can write
it WITHOUT importing handlers. The RequireAuth middleware sets the value using the
same shared key.

## Required outcome
- Delete handlers/respond.go and middleware/respond.go entirely (their contents
  move to the shared package).
- Update every call site across handlers and middleware to use the shared package.
- internal/httputil imports neither handlers nor middleware.
- `go build ./...` is clean and there is exactly ONE definition of WriteJSON,
  WriteError, the context key, and UserIDFromContext in the codebase.

## Constraints
- Do not duplicate any function. One canonical definition each.
- Do not change behavior — same JSON shape {"error","code"}, same status handling.
- Match existing naming and style.
- Do not touch business logic in handlers beyond updating the import/call references.

## Verify before finishing
Run `go build ./...` and confirm it compiles. Then grep to prove there is exactly
one definition of each moved symbol:
  grep -rn "func WriteJSON" internal/
  grep -rn "func WriteError" internal/
  grep -rn "func UserIDFromContext" internal/
Each must return exactly one result. Report the grep output.
