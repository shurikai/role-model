Project: Role Model — a self-hostable Go REST API for AI-powered resume generation.
Repository: https://github.com/shurikai/role-model

Delete internal/renderer. It's dead code: a single file containing one bare
interface (Renderer, with a Render method), zero implementations, and zero
call sites anywhere in cmd/ or internal/. It was scaffolded ahead of any
consumer — resume rendering (PDF/HTML export) isn't built yet, and per the
project's original design notes was expected to land as a separate small
service later, not as a pre-defined interface sitting unused in the main
codebase.

## End state
- Delete internal/renderer/ entirely (the whole directory)
- grep the codebase for "internal/renderer" and "renderer\." to confirm
  there are genuinely zero references before deleting (there should be
  none — this has already been verified once, but re-verify since the
  codebase has moved since then)
- Run go build ./... and go vet ./... to confirm nothing broke

## Note for whenever resume rendering actually gets built
When that session happens, don't pre-define the interface again ahead of
an implementation. Start with a concrete function in whatever package
consumes it (e.g. a method that takes resume JSON and returns bytes), and
only extract a Renderer interface if/when there's a second implementation
or an actual need to mock it in a test. This mirrors how generation,
stage0, and fitgate were all built: concrete service structs with a
shared generation.Client dependency, no speculative interfaces.

## Constraints
- This is a pure deletion — no other files should change except removing
  now-unused imports if any exist (there shouldn't be any, since nothing
  imports the package)
