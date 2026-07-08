//go:build integration

package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestProjectCRUD(t *testing.T) {
	srv, _ := testServer(t)
	suffix := uuid.NewString()
	token := signupUser(t, srv, "projcrud-"+suffix+"@test.local", "password123")

	projectID := createAndGetID(t, srv, token, "/api/v1/projects", map[string]any{
		"name":   "Widget Factory " + suffix,
		"role":   "author",
		"status": "active",
	})

	resp := doJSON(t, srv, http.MethodPatch, "/api/v1/projects/"+projectID, token, map[string]any{
		"name":   "Widget Factory Renamed " + suffix,
		"role":   "maintainer",
		"status": "dormant",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update project: got status %d", resp.StatusCode)
	}
	var updated struct {
		Name   string `json:"name"`
		Role   string `json:"role"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	resp.Body.Close()
	if updated.Name != "Widget Factory Renamed "+suffix || updated.Role != "maintainer" || updated.Status != "dormant" {
		t.Fatalf("update not reflected: got %+v", updated)
	}

	resp = doJSON(t, srv, http.MethodGet, "/api/v1/projects", token, nil)
	assertStatus(t, resp, http.StatusOK, "list projects")
	resp.Body.Close()

	resp = doJSON(t, srv, http.MethodGet, "/api/v1/projects/"+projectID, token, nil)
	assertStatus(t, resp, http.StatusOK, "get project")
	resp.Body.Close()

	resp = doJSON(t, srv, http.MethodDelete, "/api/v1/projects/"+projectID, token, nil)
	assertStatus(t, resp, http.StatusNoContent, "delete project")

	// Validation: empty name -> 400.
	resp = doJSON(t, srv, http.MethodPost, "/api/v1/projects", token, map[string]any{
		"name":   "",
		"role":   "author",
		"status": "active",
	})
	assertStatus(t, resp, http.StatusBadRequest, "create project with empty name")

	// Validation: bad role -> 400.
	resp = doJSON(t, srv, http.MethodPost, "/api/v1/projects", token, map[string]any{
		"name":   "X",
		"role":   "ceo",
		"status": "active",
	})
	assertStatus(t, resp, http.StatusBadRequest, "create project with bad role")

	// Validation: bad status -> 400.
	resp = doJSON(t, srv, http.MethodPost, "/api/v1/projects", token, map[string]any{
		"name":   "X",
		"role":   "author",
		"status": "on-fire",
	})
	assertStatus(t, resp, http.StatusBadRequest, "create project with bad status")
}

func TestProjectDelete(t *testing.T) {
	srv, _ := testServer(t)
	suffix := uuid.NewString()
	token := signupUser(t, srv, "projdel-"+suffix+"@test.local", "password123")

	projectID := createAndGetID(t, srv, token, "/api/v1/projects", map[string]any{
		"name":   "Delete Me " + suffix,
		"role":   "author",
		"status": "active",
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

	createAndGetID(t, srv, token, "/api/v1/tag-categories",
		map[string]any{"name": "Skills-" + suffix})
	tagID := createAndGetID(t, srv, token, "/api/v1/tags", map[string]any{
		"name":     "Go-" + suffix,
		"category": "Skills-" + suffix,
	})

	resp := doJSON(t, srv, http.MethodPost, "/api/v1/projects/"+projectID+"/contributions", token,
		map[string]any{"contribution_id": contribID})
	assertStatus(t, resp, http.StatusNoContent, "assign contribution to project")

	resp = doJSON(t, srv, http.MethodPost, "/api/v1/projects/"+projectID+"/tags", token,
		map[string]any{"tag_id": tagID})
	assertStatus(t, resp, http.StatusNoContent, "assign tag to project")

	resp = doJSON(t, srv, http.MethodDelete, "/api/v1/projects/"+projectID, token, nil)
	assertStatus(t, resp, http.StatusNoContent, "delete project with dependents")

	resp = doJSON(t, srv, http.MethodGet, "/api/v1/projects/"+projectID, token, nil)
	assertStatus(t, resp, http.StatusNotFound, "get deleted project")
}

func TestProjectAssignment(t *testing.T) {
	srv, _ := testServer(t)
	suffix := uuid.NewString()
	token := signupUser(t, srv, "projassign-"+suffix+"@test.local", "password123")

	projectID := createAndGetID(t, srv, token, "/api/v1/projects", map[string]any{
		"name":   "Assign Me " + suffix,
		"role":   "author",
		"status": "active",
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

	createAndGetID(t, srv, token, "/api/v1/tag-categories",
		map[string]any{"name": "Skills-" + suffix})
	tagID := createAndGetID(t, srv, token, "/api/v1/tags", map[string]any{
		"name":     "Go-" + suffix,
		"category": "Skills-" + suffix,
	})

	// Contribution assignment: assign, idempotent re-assign, unassign, unassign again.
	resp := doJSON(t, srv, http.MethodPost, "/api/v1/projects/"+projectID+"/contributions", token,
		map[string]any{"contribution_id": contribID})
	assertStatus(t, resp, http.StatusNoContent, "assign contribution")

	resp = doJSON(t, srv, http.MethodPost, "/api/v1/projects/"+projectID+"/contributions", token,
		map[string]any{"contribution_id": contribID})
	assertStatus(t, resp, http.StatusNoContent, "assign contribution again")

	resp = doJSON(t, srv, http.MethodDelete, "/api/v1/projects/"+projectID+"/contributions/"+contribID, token, nil)
	assertStatus(t, resp, http.StatusNoContent, "unassign contribution")

	resp = doJSON(t, srv, http.MethodDelete, "/api/v1/projects/"+projectID+"/contributions/"+contribID, token, nil)
	assertStatus(t, resp, http.StatusNotFound, "unassign contribution again")

	// Tag assignment: assign, idempotent re-assign, unassign, unassign again.
	resp = doJSON(t, srv, http.MethodPost, "/api/v1/projects/"+projectID+"/tags", token,
		map[string]any{"tag_id": tagID})
	assertStatus(t, resp, http.StatusNoContent, "assign tag")

	resp = doJSON(t, srv, http.MethodPost, "/api/v1/projects/"+projectID+"/tags", token,
		map[string]any{"tag_id": tagID})
	assertStatus(t, resp, http.StatusNoContent, "assign tag again")

	resp = doJSON(t, srv, http.MethodDelete, "/api/v1/projects/"+projectID+"/tags/"+tagID, token, nil)
	assertStatus(t, resp, http.StatusNoContent, "unassign tag")

	resp = doJSON(t, srv, http.MethodDelete, "/api/v1/projects/"+projectID+"/tags/"+tagID, token, nil)
	assertStatus(t, resp, http.StatusNotFound, "unassign tag again")
}

func TestProjectAssignmentIsolation(t *testing.T) {
	srv, _ := testServer(t)
	suffix := uuid.NewString()
	tokenA := signupUser(t, srv, "projisoa-"+suffix+"@test.local", "passwordA1")
	tokenB := signupUser(t, srv, "projisob-"+suffix+"@test.local", "passwordB1")

	// User A builds a project.
	projectID := createAndGetID(t, srv, tokenA, "/api/v1/projects", map[string]any{
		"name":   "A's Project " + suffix,
		"role":   "author",
		"status": "active",
	})

	// User B builds its own contribution and tag.
	employerID := createAndGetID(t, srv, tokenB, "/api/v1/employers",
		map[string]string{"name": "B's Corp " + suffix})
	positionID := createAndGetID(t, srv, tokenB, "/api/v1/positions", map[string]any{
		"employer_id": employerID,
		"title":       "Engineer",
		"started_on":  "2020-01-01",
		"sort_order":  0,
	})
	bContribID := createAndGetID(t, srv, tokenB, "/api/v1/contributions", map[string]any{
		"position_id":      positionID,
		"summary":          "Did a thing",
		"full_description": "A detailed description of the thing.",
	})
	createAndGetID(t, srv, tokenB, "/api/v1/tag-categories",
		map[string]any{"name": "Skills-" + suffix})
	bTagID := createAndGetID(t, srv, tokenB, "/api/v1/tags", map[string]any{
		"name":     "Rust-" + suffix,
		"category": "Skills-" + suffix,
	})

	// User B attempts to assign B's contribution to A's project -> 404.
	resp := doJSON(t, srv, http.MethodPost, "/api/v1/projects/"+projectID+"/contributions", tokenB,
		map[string]any{"contribution_id": bContribID})
	assertStatus(t, resp, http.StatusNotFound, "B assigns B's contribution to A's project")

	// User B attempts to assign B's tag to A's project -> 404.
	resp = doJSON(t, srv, http.MethodPost, "/api/v1/projects/"+projectID+"/tags", tokenB,
		map[string]any{"tag_id": bTagID})
	assertStatus(t, resp, http.StatusNotFound, "B assigns B's tag to A's project")

	// User A builds its own contribution; user B still can't touch A's project.
	employerIDA := createAndGetID(t, srv, tokenA, "/api/v1/employers",
		map[string]string{"name": "A's Corp 2 " + suffix})
	positionIDA := createAndGetID(t, srv, tokenA, "/api/v1/positions", map[string]any{
		"employer_id": employerIDA,
		"title":       "Engineer",
		"started_on":  "2020-01-01",
		"sort_order":  0,
	})
	aContribID := createAndGetID(t, srv, tokenA, "/api/v1/contributions", map[string]any{
		"position_id":      positionIDA,
		"summary":          "Did a thing",
		"full_description": "A detailed description of the thing.",
	})
	resp = doJSON(t, srv, http.MethodPost, "/api/v1/projects/"+projectID+"/contributions", tokenB,
		map[string]any{"contribution_id": aContribID})
	assertStatus(t, resp, http.StatusNotFound, "B assigns A's contribution to A's project")
}
