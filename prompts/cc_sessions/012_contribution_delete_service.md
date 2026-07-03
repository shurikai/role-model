Project: Role Model — a self-hostable Go REST API for AI-powered resume generation.
Repository: https://github.com/shurikai/role-model

Move ContributionHandler.Delete's transaction out of the HTTP handler and
into a new, narrowly-scoped internal/contribution package. This is a
refactor only — no behavior change, no new endpoints, no schema change.
Same class of fix as 008_approve_draft_service_method.md, applied to the
one other handler that still owns a multi-step DB transaction directly.

## Problem
ContributionHandler.Delete currently does its own tx.Begin -> three
sequential deletes (DeleteContributionTags, DeleteContributionProjectLinks,
DeleteContribution) -> commit, all inline in the HTTP handler. Per this
project's own architectural principle ("business logic not in handlers"),
that orchestration belongs in a service layer, matching how
stage0.Service.ApproveDraft and stage0.Service.insertDrafts already do
this kind of multi-step DB work.

## Scope note — do not over-build this
Employer, Position, and the rest of Contribution's own CRUD methods
(Create/Update/Get/ListByPosition) are simple single-query pass-throughs
today and do NOT need a service layer — extracting those into a service
package would be adding structure with no real orchestration behind it,
the same mistake the deleted internal/renderer package made. Only the
cascading-delete logic gets extracted here. Do not create service methods
for the other Contribution CRUD operations, and do not touch
Employer/Position handlers in this session.

## End state

### internal/contribution/service.go (new package)
    package contribution

    type Service struct {
        pool    *pgxpool.Pool
        queries *db.Queries
    }

    func NewService(pool *pgxpool.Pool, queries *db.Queries) *Service {
        return &Service{pool: pool, queries: queries}
    }

    var ErrNotFound = errors.New("contribution not found")

    func (s *Service) Delete(ctx context.Context, userID, contributionID uuid.UUID) error

Behavior (moved verbatim from the handler, no logic changes):
- Ownership check via s.queries.GetContribution(ctx, db.GetContributionParams{
  ID: contributionID, UserID: userID}); on pgx.ErrNoRows return ErrNotFound
- Begin a transaction on s.pool, defer rollback
- qtx := s.queries.WithTx(tx)
- qtx.DeleteContributionTags(ctx, contributionID)
- qtx.DeleteContributionProjectLinks(ctx, contributionID)
- qtx.DeleteContribution(ctx, db.DeleteContributionParams{ID: contributionID,
  UserID: userID})
- Commit, wrapping any step's error with fmt.Errorf("%w: ...", err) style
  consistent with stage0/service.go

### internal/api/handlers/contribution.go
- Add a contribSvc *contribution.Service field to ContributionHandler and
  a constructor param, following the exact pattern ImportHandler uses for
  its stage0Svc field
- Replace the body of ContributionHandler.Delete so it only:
  1. Extracts userID from context
  2. Parses id from the URL param
  3. Calls h.contribSvc.Delete(r.Context(), userID, id)
  4. Maps errors: contribution.ErrNotFound -> 404 not_found, anything
     else -> 500 internal_error "failed to delete contribution"
  5. On success, respond 204 with no body (matches current behavior)
- h.pool is no longer needed by ContributionHandler once this lands if
  nothing else in the file uses it directly — remove the field and
  constructor param only if grep confirms it's unused elsewhere in the
  file (Create/Update/Get/ListByPosition should all be queries-only)

### internal/api/router.go / cmd/server/main.go
Wire the new contribution.Service the same way stage0.Service and
fitgate.Service are already wired: construct it in main.go, pass it
through RouterDeps, pass it into NewContributionHandler.

## Constraints
- No behavior change visible to API consumers — same route, same status
  codes, same response shape
- Do not add service methods for Create/Update/Get/ListByPosition — see
  scope note above
- Keep error wrapping style consistent with internal/stage0/service.go
- Run go build ./... and go vet ./... before finishing
