Context: Role Model, Go service. Read CLAUDE.md and these existing files first to
match patterns exactly:
- internal/api/handlers/education.go (flat CRUD handler pattern)
- internal/api/handlers/contribution.go (parent-ownership check pattern)
- internal/db/queries/education.sql (query style)
- internal/api/router.go (route registration, protected group)
- internal/api/api_test.go (integration test harness + helpers)

Task: Implement full write CRUD for tags and tag_categories, plus tag-assignment
endpoints for contributions. This is the last new backend pattern before projects.

## Queries (internal/db/queries/tags.sql — add to existing)
-- name: CreateTagCategory :one
INSERT INTO tag_categories (id, user_id, name, sort_order)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListTagCategories :many
SELECT * FROM tag_categories
WHERE user_id = $1
ORDER BY sort_order, name;

-- name: DeleteTagCategory :execrows
DELETE FROM tag_categories
WHERE id = $1 AND user_id = $2;

-- name: CountTagsInCategory :one
SELECT count(*) FROM tags
WHERE user_id = $1 AND category = $2;

-- name: CreateTag :one
INSERT INTO tags (id, user_id, name, aliases, category, sort_order)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListTags :many
SELECT * FROM tags
WHERE user_id = $1
ORDER BY sort_order, name;

-- name: GetTag :one
SELECT * FROM tags
WHERE id = $1 AND user_id = $2;

-- name: DeleteTag :execrows
DELETE FROM tags
WHERE id = $1 AND user_id = $2;

-- name: CountTagUsage :one
SELECT
  (SELECT count(*) FROM contribution_tags WHERE tag_id = $1) +
  (SELECT count(*) FROM project_tags WHERE tag_id = $1) AS total;

-- name: AssignTagToContribution :exec
INSERT INTO contribution_tags (contribution_id, tag_id)
VALUES ($1, $2)
ON CONFLICT (contribution_id, tag_id) DO NOTHING;

-- name: UnassignTagFromContribution :execrows
DELETE FROM contribution_tags
WHERE contribution_id = $1 AND tag_id = $2;

Run sqlc generate after adding.

## Handlers (internal/api/handlers/tag.go — new file)
TagHandler holds *db.Queries only (no pool — no transactions needed here).

Plain CRUD (mirror education.go exactly):
- CreateTagCategory: name required, sort_order optional (default 0). 201.
- ListTagCategories: return the user's categories. 200.
- DeleteTagCategory: guard with CountTagsInCategory — if >0, 409 has_dependents
  ("category has tags and cannot be deleted"). Else DeleteTagCategory (:execrows),
  0 rows -> 404, 1 row -> 204.
- CreateTag: name + category required; aliases optional ([]string -> pgtype/[]string
  per generated type); sort_order optional. The category must exist for this user —
  verify via a category existence check OR rely on the composite FK; if relying on
  FK, catch the FK violation and return 400 "category does not exist". 201.
- ListTags: return the user's tags. 200.
- DeleteTag: guard with CountTagUsage — if >0, 409 has_dependents ("tag is assigned
  and cannot be deleted"). Else DeleteTag (:execrows), 0 -> 404, 1 -> 204.

Assignment (the NEW pattern — double-ownership check):
- AssignToContribution: POST /contributions/{id}/tags, body {"tag_id":"uuid"}.
  Verify BOTH the contribution AND the tag belong to the authenticated user
  (GetContribution scoped to userID, then GetTag scoped to userID; each 404s if
  not owned). Then AssignTagToContribution (conflict-tolerant, idempotent). 204.
- UnassignFromContribution: DELETE /contributions/{id}/tags/{tagId}. Parse both
  ids. UnassignTagFromContribution (:execrows). 0 rows -> 404, 1 -> 204.
  (No body. tagId from the URL path.)

Use the AssignToContribution handler below as the
reference implementation for the assignment double-ownership pattern.

```
func (h *TagHandler) AssignToContribution(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	contribID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "contribution id must be a valid UUID")
		return
	}

	var req struct {
		TagID uuid.UUID `json:"tag_id"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	// Double-ownership: BOTH the contribution and the tag must belong to this user.
	if _, err := h.queries.GetContribution(r.Context(), db.GetContributionParams{ID: contribID, UserID: userID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "contribution not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to verify contribution")
		return
	}
	if _, err := h.queries.GetTag(r.Context(), db.GetTagParams{ID: req.TagID, UserID: userID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "tag not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to verify tag")
		return
	}

	if err := h.queries.AssignTagToContribution(r.Context(), db.AssignTagToContributionParams{
		ContributionID: contribID,
		TagID:          req.TagID,
	}); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to assign tag")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
```

## Routes (internal/api/router.go — protected group)
tagHandler := handlers.NewTagHandler(queries)
r.Post("/tags", tagHandler.Create)
r.Get("/tags", tagHandler.List)
r.Delete("/tags/{id}", tagHandler.Delete)

r.Post("/tag-categories", tagHandler.CreateCategory)
r.Get("/tag-categories", tagHandler.ListCategories)
r.Delete("/tag-categories/{id}", tagHandler.DeleteCategory)

r.Post("/contributions/{id}/tags", tagHandler.AssignToContribution)
r.Delete("/contributions/{id}/tags/{tagId}", tagHandler.UnassignFromContribution)

## Tests (internal/api/ — extend api_test.go or a new tag_test.go, //go:build integration)
Use the existing helpers (testServer, signupUser, createAndGetID, doJSON,
assertStatus). CRITICAL: testServer builds its OWN RouterDeps literal — if
TagHandler needs anything new in Deps, add it there too (this exact omission
caused a nil-service panic last session; do not repeat it).

- TestTagCRUD: create category, create tag in it, list tags, delete tag (204),
  delete category (204). Validation: empty name -> 400.
- TestTagDeleteGuard: create tag, assign it to a contribution, attempt delete ->
  409 has_dependents. Unassign, then delete -> 204.
- TestTagAssignment: create contribution + tag, assign (204), assign again
  (idempotent, still 204), unassign (204), unassign again (404).
- TestTagAssignmentIsolation: two users. User B attempts to assign B's tag to A's
  contribution -> 404. User B attempts to assign a tag to A's contribution ->
  404. (Double-ownership enforced.)

## Constraints
- Match education.go / contribution.go patterns exactly (userID from context via
  httputil.UserIDFromContext; httputil.WriteError/WriteJSON; :execrows deletes
  returning 404 on 0 rows).
- Use createAndGetID for creates in tests (reads body once, prints body on
  failure — do NOT use separate assertStatus+idFrom on one response).
- Unique emails per test run via uuid suffix.
- Do not modify existing handlers, existing queries, or the schema.
- Add List/read endpoints as specified (the frontend will need them).

## Verify before finishing
Run: go clean -testcache && go test -tags integration -run TestTag \
  ./internal/api/... -v  (with DATABASE_URL and JWT_SECRET set).
Report PASS/FAIL per test. If any fail, print the failing assertion AND the
response body before changing any test logic. Do not relax assertions to force
green — report and ask.
