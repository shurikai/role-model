//go:build integration

package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// decodeOK asserts the status and decodes the body in one pass.
//
// assertStatus closes the body, so the two cannot be composed — and a test
// that asserts only the status code is the shape #93 caught passing while
// asserting nothing.
func decodeOK(t *testing.T, resp *http.Response, want int, label string, out any) {
	t.Helper()
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("%s: read body: %v", label, err)
	}
	if resp.StatusCode != want {
		t.Fatalf("%s: got status %d, want %d, body: %s", label, resp.StatusCode, want, body)
	}
	if out == nil {
		return
	}
	if err := json.Unmarshal(body, out); err != nil {
		t.Fatalf("%s: decode body: %v (%s)", label, err, body)
	}
}

func listPreferences(t *testing.T, srv *httptest.Server, token string) []struct {
	ID         string   `json:"id"`
	Label      string   `json:"label"`
	Aliases    []string `json:"aliases"`
	Weight     int      `json:"weight"`
	IsHardGate bool     `json:"is_hard_gate"`
} {
	t.Helper()
	var out []struct {
		ID         string   `json:"id"`
		Label      string   `json:"label"`
		Aliases    []string `json:"aliases"`
		Weight     int      `json:"weight"`
		IsHardGate bool     `json:"is_hard_gate"`
	}
	resp := doJSON(t, srv, http.MethodGet, "/api/v1/preferences", token, nil)
	decodeOK(t, resp, http.StatusOK, "list preferences", &out)
	return out
}

// The hole this closes: preferences drive half the fit gate, and the intake
// resolver was the only writer of one. A new account could state a preference
// exactly once, through an import, and never correct it — so the preference
// axis scored against whatever an extractor happened to propose, forever.
func TestPreferenceCRUD(t *testing.T) {
	srv, _ := testServer(t)

	suffix := uuid.NewString()
	token := signupUser(t, srv, "preference-"+suffix+"@test.local", "password123")

	prefID := createAndGetID(t, srv, token, "/api/v1/preferences", map[string]any{
		"preference_type": "dealbreaker",
		"label":           "inpatient nights",
		"aliases":         []string{"night shift", "overnight rotation", "night float"},
		"sentiment":       "negative",
		"weight":          10,
		"is_hard_gate":    true,
	})

	listed := listPreferences(t, srv, token)
	if len(listed) != 1 {
		t.Fatalf("preferences = %d, want 1", len(listed))
	}

	// The assertion that matters. CreatePreference did not insert this column
	// at all until #89 — the row was written, the caller was handed back a
	// body that looked right, and the fit gate then matched on the label
	// alone. Aliases are what let "inpatient nights" reach a posting reading
	// "three twelve-hour night shifts".
	if len(listed[0].Aliases) != 3 {
		t.Errorf("aliases = %v, want three; a NULL here means the column was dropped again", listed[0].Aliases)
	}
	if !listed[0].IsHardGate {
		t.Error("is_hard_gate did not survive the round trip")
	}

	var updated struct {
		Weight     int      `json:"weight"`
		Aliases    []string `json:"aliases"`
		IsHardGate bool     `json:"is_hard_gate"`
	}
	resp := doJSON(t, srv, http.MethodPatch, "/api/v1/preferences/"+prefID, token, map[string]any{
		"preference_type": "dealbreaker",
		"label":           "inpatient nights",
		"aliases":         []string{"night shift", "nocturnist"},
		"sentiment":       "negative",
		"weight":          8,
		"is_hard_gate":    false,
	})
	decodeOK(t, resp, http.StatusOK, "update preference", &updated)
	if updated.Weight != 8 || updated.IsHardGate {
		t.Errorf("update did not take: weight=%d is_hard_gate=%v", updated.Weight, updated.IsHardGate)
	}
	// UpdatePreference had the same missing column as CreatePreference.
	if len(updated.Aliases) != 2 {
		t.Errorf("aliases after update = %v, want two", updated.Aliases)
	}

	resp = doJSON(t, srv, http.MethodDelete, "/api/v1/preferences/"+prefID, token, nil)
	assertStatus(t, resp, http.StatusNoContent, "delete preference")

	// DeletePreference was :exec, which returns no row count — so a delete of
	// something that does not exist looked identical to a successful one, and
	// a request carrying another user's id would have answered 204.
	resp = doJSON(t, srv, http.MethodDelete, "/api/v1/preferences/"+prefID, token, nil)
	assertStatus(t, resp, http.StatusNotFound, "delete an already-deleted preference")
}

// The two rejected vocabularies here are not arbitrary typos: "work_type" and
// "hard_exclude" are the names migrations 021 and 011 removed. Either should
// come back as a 400 naming the field rather than a 500 wrapping a constraint
// violation.
func TestPreferenceValidation(t *testing.T) {
	srv, _ := testServer(t)

	suffix := uuid.NewString()
	token := signupUser(t, srv, "prefvalid-"+suffix+"@test.local", "password123")

	cases := []struct {
		name  string
		mutis func(map[string]any)
	}{
		{"blank label", func(m map[string]any) { m["label"] = "   " }},
		{"retired preference_type", func(m map[string]any) { m["preference_type"] = "work_type" }},
		{"retired sentiment", func(m map[string]any) { m["sentiment"] = "hard_exclude" }},
		{"weight below range", func(m map[string]any) { m["weight"] = 0 }},
		{"weight above range", func(m map[string]any) { m["weight"] = 11 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]any{
				"preference_type": "domain",
				"label":           "ambulatory quality improvement",
				"sentiment":       "positive",
				"weight":          7,
				"is_hard_gate":    false,
			}
			tc.mutis(body)
			resp := doJSON(t, srv, http.MethodPost, "/api/v1/preferences", token, body)
			assertStatus(t, resp, http.StatusBadRequest, tc.name)
		})
	}

	// And nothing was written by any of them.
	if got := listPreferences(t, srv, token); len(got) != 0 {
		t.Errorf("a rejected request still wrote a row: %+v", got)
	}
}

type listedSkill struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	IsActive bool   `json:"is_active"`
}

func TestSkillCRUD(t *testing.T) {
	srv, _ := testServer(t)

	suffix := uuid.NewString()
	token := signupUser(t, srv, "skill-"+suffix+"@test.local", "password123")

	// The category does not exist yet. Creating the skill has to build the
	// tag_categories -> tags -> skills chain in order, which is what
	// ResolveOrCreateTag is for.
	skillID := createAndGetID(t, srv, token, "/api/v1/skills", map[string]any{
		"category":         "Charting Systems",
		"tag":              "Epic",
		"proficiency":      "proficient",
		"years_experience": 6.5,
	})

	var listed []listedSkill
	resp := doJSON(t, srv, http.MethodGet, "/api/v1/skills", token, nil)
	decodeOK(t, resp, http.StatusOK, "list skills", &listed)
	if len(listed) != 1 {
		t.Fatalf("skills = %d, want 1", len(listed))
	}
	if listed[0].Name != "Epic" || listed[0].Category != "Charting Systems" {
		t.Errorf("list dropped the tag behind the skill: %+v", listed[0])
	}

	// The existing spelling wins, so the same tag under a different
	// capitalisation must resolve to the tag already there — forking it would
	// split the evidence for every requirement either spelling answers.
	//
	// The request is then refused, because skills is UNIQUE (user_id, tag_id):
	// a person has one depth for a thing. A 409 saying so is the useful
	// answer; the 500 this returned before is not, and silently upserting
	// would overwrite a depth they set earlier without saying it had.
	resp = doJSON(t, srv, http.MethodPost, "/api/v1/skills", token, map[string]any{
		"category":    "charting systems",
		"tag":         "epic",
		"proficiency": "novice",
	})
	assertStatus(t, resp, http.StatusConflict, "duplicate skill on the same tag")

	var tags []struct {
		Name string `json:"name"`
	}
	resp = doJSON(t, srv, http.MethodGet, "/api/v1/tags", token, nil)
	decodeOK(t, resp, http.StatusOK, "list tags", &tags)
	if len(tags) != 1 {
		t.Errorf("tags = %v, want one; a case difference forked the vocabulary", tags)
	}
	if tags[0].Name != "Epic" {
		t.Errorf("tag name = %q, want the existing spelling %q to win", tags[0].Name, "Epic")
	}

	// And the original skill kept the depth it was created with.
	resp = doJSON(t, srv, http.MethodGet, "/api/v1/skills", token, nil)
	decodeOK(t, resp, http.StatusOK, "re-list after the refused duplicate", &listed)
	if len(listed) != 1 {
		t.Errorf("skills = %d, want the duplicate to have written nothing", len(listed))
	}

	// Deactivating is the main thing this screen is for, and the list must
	// still show what was just deactivated or there is no way to put it back.
	resp = doJSON(t, srv, http.MethodPatch, "/api/v1/skills/"+skillID, token, map[string]any{
		"proficiency": "expert",
		"is_active":   false,
	})
	assertStatus(t, resp, http.StatusOK, "update skill")

	resp = doJSON(t, srv, http.MethodGet, "/api/v1/skills", token, nil)
	decodeOK(t, resp, http.StatusOK, "re-list skills", &listed)
	var found bool
	for _, s := range listed {
		if s.ID == skillID {
			found = true
			if s.IsActive {
				t.Error("is_active did not take")
			}
		}
	}
	if !found {
		t.Error("a deactivated skill vanished from the list, so nothing can reactivate it")
	}

	resp = doJSON(t, srv, http.MethodDelete, "/api/v1/skills/"+skillID, token, nil)
	assertStatus(t, resp, http.StatusNoContent, "delete skill")
	resp = doJSON(t, srv, http.MethodDelete, "/api/v1/skills/"+skillID, token, nil)
	assertStatus(t, resp, http.StatusNotFound, "delete an already-deleted skill")

	// Deleting a skill must not take the tag with it. Tags are vocabulary
	// shared with contributions; removing one because a claim was retracted
	// would strip the term off every contribution carrying it.
	resp = doJSON(t, srv, http.MethodGet, "/api/v1/tags", token, nil)
	decodeOK(t, resp, http.StatusOK, "list tags after delete", &tags)
	if len(tags) != 1 {
		t.Errorf("tags = %v, want the tag to survive the skill", tags)
	}
}

// Proficiency is validated against the account's OWN proficiency_levels rows.
// Migration 020 made that scale user-owned and dropped the CHECK; validating
// against a Go constant would put back the seventh copy of a vocabulary the
// neutrality work spent six migrations removing.
func TestSkillProficiencyIsCheckedAgainstTheAccountsOwnScale(t *testing.T) {
	srv, _ := testServer(t)

	suffix := uuid.NewString()
	token := signupUser(t, srv, "prof-"+suffix+"@test.local", "password123")

	var apiErr struct {
		Error string `json:"error"`
	}
	resp := doJSON(t, srv, http.MethodPost, "/api/v1/skills", token, map[string]any{
		"category":    "Charting Systems",
		"tag":         "Epic",
		"proficiency": "world-class",
	})
	decodeOK(t, resp, http.StatusBadRequest, "a level not on the account's scale", &apiErr)

	// The message has to name what IS allowed. "invalid proficiency" leaves
	// the caller guessing at a vocabulary that is per-account by design.
	for _, want := range []string{"novice", "proficient", "expert"} {
		if !strings.Contains(apiErr.Error, want) {
			t.Errorf("error %q does not name the allowed level %q", apiErr.Error, want)
		}
	}
}
