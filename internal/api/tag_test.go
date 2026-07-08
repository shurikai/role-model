//go:build integration

package api_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestTagCRUD(t *testing.T) {
	srv, _ := testServer(t)
	suffix := uuid.NewString()
	token := signupUser(t, srv, "tagcrud-"+suffix+"@test.local", "password123")

	categoryID := createAndGetID(t, srv, token, "/api/v1/tag-categories",
		map[string]any{"name": "Languages-" + suffix})

	tagID := createAndGetID(t, srv, token, "/api/v1/tags", map[string]any{
		"name":     "Go-" + suffix,
		"category": "Languages-" + suffix,
	})

	resp := doJSON(t, srv, http.MethodGet, "/api/v1/tags", token, nil)
	assertStatus(t, resp, http.StatusOK, "list tags")
	resp.Body.Close()

	resp = doJSON(t, srv, http.MethodDelete, "/api/v1/tags/"+tagID, token, nil)
	assertStatus(t, resp, http.StatusNoContent, "delete tag")

	resp = doJSON(t, srv, http.MethodDelete, "/api/v1/tag-categories/"+categoryID, token, nil)
	assertStatus(t, resp, http.StatusNoContent, "delete tag category")

	// Validation: empty name -> 400.
	resp = doJSON(t, srv, http.MethodPost, "/api/v1/tag-categories", token,
		map[string]any{"name": ""})
	assertStatus(t, resp, http.StatusBadRequest, "create category with empty name")
}

func TestTagDeleteGuard(t *testing.T) {
	srv, _ := testServer(t)
	suffix := uuid.NewString()
	token := signupUser(t, srv, "tagguard-"+suffix+"@test.local", "password123")

	createAndGetID(t, srv, token, "/api/v1/tag-categories",
		map[string]any{"name": "Skills-" + suffix})

	tagID := createAndGetID(t, srv, token, "/api/v1/tags", map[string]any{
		"name":     "Postgres-" + suffix,
		"category": "Skills-" + suffix,
	})

	employerID := createAndGetID(t, srv, token, "/api/v1/employers",
		map[string]string{"name": "Test Corp " + suffix})

	positionID := createAndGetID(t, srv, token, "/api/v1/positions", map[string]any{
		"employer_id": employerID,
		"title":       "Engineer",
		"started_on":  "2020-01-01",
		"sort_order":  0,
	})

	contribID := createAndGetID(t, srv, token, "/api/v1/contributions", map[string]any{
		"position_id":      positionID,
		"summary":          "Did a thing",
		"full_description": "A detailed description of the thing.",
	})

	resp := doJSON(t, srv, http.MethodPost, "/api/v1/contributions/"+contribID+"/tags", token,
		map[string]any{"tag_id": tagID})
	assertStatus(t, resp, http.StatusNoContent, "assign tag")

	resp = doJSON(t, srv, http.MethodDelete, "/api/v1/tags/"+tagID, token, nil)
	assertStatus(t, resp, http.StatusConflict, "delete tag while assigned")

	resp = doJSON(t, srv, http.MethodDelete, "/api/v1/contributions/"+contribID+"/tags/"+tagID, token, nil)
	assertStatus(t, resp, http.StatusNoContent, "unassign tag")

	resp = doJSON(t, srv, http.MethodDelete, "/api/v1/tags/"+tagID, token, nil)
	assertStatus(t, resp, http.StatusNoContent, "delete tag after unassign")
}

func TestTagAssignment(t *testing.T) {
	srv, _ := testServer(t)
	suffix := uuid.NewString()
	token := signupUser(t, srv, "tagassign-"+suffix+"@test.local", "password123")

	createAndGetID(t, srv, token, "/api/v1/tag-categories",
		map[string]any{"name": "Skills-" + suffix})

	tagID := createAndGetID(t, srv, token, "/api/v1/tags", map[string]any{
		"name":     "Kubernetes-" + suffix,
		"category": "Skills-" + suffix,
	})

	employerID := createAndGetID(t, srv, token, "/api/v1/employers",
		map[string]string{"name": "Test Corp " + suffix})

	positionID := createAndGetID(t, srv, token, "/api/v1/positions", map[string]any{
		"employer_id": employerID,
		"title":       "Engineer",
		"started_on":  "2020-01-01",
		"sort_order":  0,
	})

	contribID := createAndGetID(t, srv, token, "/api/v1/contributions", map[string]any{
		"position_id":      positionID,
		"summary":          "Did a thing",
		"full_description": "A detailed description of the thing.",
	})

	resp := doJSON(t, srv, http.MethodPost, "/api/v1/contributions/"+contribID+"/tags", token,
		map[string]any{"tag_id": tagID})
	assertStatus(t, resp, http.StatusNoContent, "assign tag")

	// Idempotent: assigning again still succeeds.
	resp = doJSON(t, srv, http.MethodPost, "/api/v1/contributions/"+contribID+"/tags", token,
		map[string]any{"tag_id": tagID})
	assertStatus(t, resp, http.StatusNoContent, "assign tag again")

	resp = doJSON(t, srv, http.MethodDelete, "/api/v1/contributions/"+contribID+"/tags/"+tagID, token, nil)
	assertStatus(t, resp, http.StatusNoContent, "unassign tag")

	resp = doJSON(t, srv, http.MethodDelete, "/api/v1/contributions/"+contribID+"/tags/"+tagID, token, nil)
	assertStatus(t, resp, http.StatusNotFound, "unassign tag again")
}

func TestTagAssignmentIsolation(t *testing.T) {
	srv, _ := testServer(t)
	suffix := uuid.NewString()
	tokenA := signupUser(t, srv, "tagisoa-"+suffix+"@test.local", "passwordA1")
	tokenB := signupUser(t, srv, "tagisob-"+suffix+"@test.local", "passwordB1")

	// User A builds a contribution.
	employerID := createAndGetID(t, srv, tokenA, "/api/v1/employers",
		map[string]string{"name": "A's Corp " + suffix})
	positionID := createAndGetID(t, srv, tokenA, "/api/v1/positions", map[string]any{
		"employer_id": employerID,
		"title":       "Engineer",
		"started_on":  "2020-01-01",
		"sort_order":  0,
	})
	contribID := createAndGetID(t, srv, tokenA, "/api/v1/contributions", map[string]any{
		"position_id":      positionID,
		"summary":          "Did a thing",
		"full_description": "A detailed description of the thing.",
	})

	// User B builds its own tag.
	createAndGetID(t, srv, tokenB, "/api/v1/tag-categories",
		map[string]any{"name": "Skills-" + suffix})
	bTagID := createAndGetID(t, srv, tokenB, "/api/v1/tags", map[string]any{
		"name":     "Rust-" + suffix,
		"category": "Skills-" + suffix,
	})

	// User B attempts to assign B's tag to A's contribution -> 404.
	resp := doJSON(t, srv, http.MethodPost, "/api/v1/contributions/"+contribID+"/tags", tokenB,
		map[string]any{"tag_id": bTagID})
	assertStatus(t, resp, http.StatusNotFound, "B assigns B's tag to A's contribution")

	// User A builds its own tag; user B attempts to assign it to A's contribution -> 404.
	createAndGetID(t, srv, tokenA, "/api/v1/tag-categories",
		map[string]any{"name": "Languages-" + suffix})
	aTagID := createAndGetID(t, srv, tokenA, "/api/v1/tags", map[string]any{
		"name":     "Go-" + suffix,
		"category": "Languages-" + suffix,
	})
	resp = doJSON(t, srv, http.MethodPost, "/api/v1/contributions/"+contribID+"/tags", tokenB,
		map[string]any{"tag_id": aTagID})
	assertStatus(t, resp, http.StatusNotFound, "B assigns A's tag to A's contribution")
}
