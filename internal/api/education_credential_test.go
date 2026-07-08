//go:build integration

package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestEducationCRUD(t *testing.T) {
	srv, _ := testServer(t)

	suffix := uuid.NewString()
	token := signupUser(t, srv, "education-"+suffix+"@test.local", "password123")

	degree := "BS Computer Science"
	eduID := createAndGetID(t, srv, token, "/api/v1/education", map[string]any{
		"institution":    "Lakeside State University",
		"degree":         degree,
		"field_of_study": "Computer Science",
		"started_on":     "2010-08-01",
		"ended_on":       "2014-05-01",
		"notes":          "Graduated cum laude",
	})

	resp := doJSON(t, srv, http.MethodGet, "/api/v1/education", token, nil)
	assertStatus(t, resp, http.StatusOK, "list education")

	// Update (full replace) with changed fields.
	resp = doJSON(t, srv, http.MethodPatch, "/api/v1/education/"+eduID, token, map[string]any{
		"institution":    "Lakeside State University",
		"degree":         "MS Computer Science",
		"field_of_study": "Computer Science",
		"started_on":     "2010-08-01",
		"ended_on":       "2016-05-01",
		"notes":          "Graduated with honors",
	})
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("update education: status %d, body: %s", resp.StatusCode, b)
	}
	var updated struct {
		Degree string `json:"degree"`
		Notes  string `json:"notes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		t.Fatalf("decode updated education: %v", err)
	}
	resp.Body.Close()
	if updated.Degree != "MS Computer Science" {
		t.Errorf("update education: got degree %q, want %q", updated.Degree, "MS Computer Science")
	}
	if updated.Notes != "Graduated with honors" {
		t.Errorf("update education: got notes %q, want %q", updated.Notes, "Graduated with honors")
	}

	// Delete.
	resp = doJSON(t, srv, http.MethodDelete, "/api/v1/education/"+eduID, token, nil)
	assertStatus(t, resp, http.StatusNoContent, "delete education")

	// Confirm it's gone from the list.
	resp = doJSON(t, srv, http.MethodGet, "/api/v1/education", token, nil)
	assertNotInList(t, resp, eduID, "education")

	// Validation: empty institution.
	resp = doJSON(t, srv, http.MethodPost, "/api/v1/education", token, map[string]any{
		"institution": "",
	})
	assertStatus(t, resp, http.StatusBadRequest, "create education with empty institution")

	// Date validation: malformed started_on.
	resp = doJSON(t, srv, http.MethodPost, "/api/v1/education", token, map[string]any{
		"institution": "Some University",
		"started_on":  "not-a-date",
	})
	assertStatus(t, resp, http.StatusBadRequest, "create education with malformed started_on")
}

func TestCredentialCRUD(t *testing.T) {
	srv, _ := testServer(t)

	suffix := uuid.NewString()
	token := signupUser(t, srv, "credential-"+suffix+"@test.local", "password123")

	credID := createAndGetID(t, srv, token, "/api/v1/credentials", map[string]any{
		"name":           "AWS Certified Solutions Architect",
		"issuer":         "Amazon Web Services",
		"issued_on":      "2021-03-01",
		"expires_on":     "2024-03-01",
		"credential_url": "https://example.com/cred/123",
	})

	resp := doJSON(t, srv, http.MethodGet, "/api/v1/credentials", token, nil)
	assertStatus(t, resp, http.StatusOK, "list credentials")

	// Update (full replace) with changed fields.
	resp = doJSON(t, srv, http.MethodPatch, "/api/v1/credentials/"+credID, token, map[string]any{
		"name":           "AWS Certified Solutions Architect - Professional",
		"issuer":         "Amazon Web Services",
		"issued_on":      "2021-03-01",
		"expires_on":     "2027-03-01",
		"credential_url": "https://example.com/cred/123",
	})
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("update credential: status %d, body: %s", resp.StatusCode, b)
	}
	var updated struct {
		Name      string `json:"name"`
		ExpiresOn string `json:"expires_on"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		t.Fatalf("decode updated credential: %v", err)
	}
	resp.Body.Close()
	if updated.Name != "AWS Certified Solutions Architect - Professional" {
		t.Errorf("update credential: got name %q, want %q", updated.Name, "AWS Certified Solutions Architect - Professional")
	}
	if updated.ExpiresOn != "2027-03-01" {
		t.Errorf("update credential: got expires_on %q, want %q", updated.ExpiresOn, "2027-03-01")
	}

	// Delete.
	resp = doJSON(t, srv, http.MethodDelete, "/api/v1/credentials/"+credID, token, nil)
	assertStatus(t, resp, http.StatusNoContent, "delete credential")

	// Confirm it's gone from the list.
	resp = doJSON(t, srv, http.MethodGet, "/api/v1/credentials", token, nil)
	assertNotInList(t, resp, credID, "credential")

	// Validation: empty name.
	resp = doJSON(t, srv, http.MethodPost, "/api/v1/credentials", token, map[string]any{
		"name": "",
	})
	assertStatus(t, resp, http.StatusBadRequest, "create credential with empty name")

	// Date validation: malformed issued_on.
	resp = doJSON(t, srv, http.MethodPost, "/api/v1/credentials", token, map[string]any{
		"name":      "Some Credential",
		"issued_on": "not-a-date",
	})
	assertStatus(t, resp, http.StatusBadRequest, "create credential with malformed issued_on")
}

func TestEducationCredentialsIsolation(t *testing.T) {
	srv, _ := testServer(t)

	suffix := uuid.NewString()
	tokenA := signupUser(t, srv, "eduiso-a-"+suffix+"@test.local", "passwordA1")
	tokenB := signupUser(t, srv, "eduiso-b-"+suffix+"@test.local", "passwordB1")

	// User A creates an education entry and a credential.
	eduID := createAndGetID(t, srv, tokenA, "/api/v1/education", map[string]any{
		"institution": "A's University",
	})
	credID := createAndGetID(t, srv, tokenA, "/api/v1/credentials", map[string]any{
		"name": "A's Credential",
	})

	// User B must NOT be able to update A's education.
	resp := doJSON(t, srv, http.MethodPatch, "/api/v1/education/"+eduID, tokenB,
		map[string]any{"institution": "Hijacked"})
	assertStatus(t, resp, http.StatusNotFound, "B updates A's education")

	// User B must NOT be able to delete A's education.
	resp = doJSON(t, srv, http.MethodDelete, "/api/v1/education/"+eduID, tokenB, nil)
	assertStatus(t, resp, http.StatusNotFound, "B deletes A's education")

	// User B must NOT be able to update A's credential.
	resp = doJSON(t, srv, http.MethodPatch, "/api/v1/credentials/"+credID, tokenB,
		map[string]any{"name": "Hijacked"})
	assertStatus(t, resp, http.StatusNotFound, "B updates A's credential")

	// User B must NOT be able to delete A's credential.
	resp = doJSON(t, srv, http.MethodDelete, "/api/v1/credentials/"+credID, tokenB, nil)
	assertStatus(t, resp, http.StatusNotFound, "B deletes A's credential")

	// User B's lists must NOT include A's rows.
	resp = doJSON(t, srv, http.MethodGet, "/api/v1/education", tokenB, nil)
	assertNotInList(t, resp, eduID, "education")
	resp = doJSON(t, srv, http.MethodGet, "/api/v1/credentials", tokenB, nil)
	assertNotInList(t, resp, credID, "credential")
}

// assertNotInList decodes a list response body ([{"id": "..."}]) and fails if
// wantAbsentID appears among the returned ids.
func assertNotInList(t *testing.T, resp *http.Response, wantAbsentID, label string) {
	t.Helper()
	defer resp.Body.Close()
	var items []struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		t.Fatalf("decode %s list: %v", label, err)
	}
	for _, item := range items {
		if item.ID == wantAbsentID {
			t.Errorf("%s list still contains id %s", label, wantAbsentID)
			return
		}
	}
}
