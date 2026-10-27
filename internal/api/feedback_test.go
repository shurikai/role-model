//go:build integration

package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/shurikai/role-model/internal/config"
	"github.com/shurikai/role-model/internal/db"
)

func validFeedback() map[string]any {
	return map[string]any{
		"category":    "confusing",
		"message":     "I couldn't find where to fix a job title.",
		"route":       "/career/positions/abc?tab=1",
		"app_version": "a1b2c3d",
		"viewport":    "1440x900",
	}
}

func with(body map[string]any, key string, value any) map[string]any {
	out := make(map[string]any, len(body))
	for k, v := range body {
		out[k] = v
	}
	if value == nil {
		delete(out, key)
	} else {
		out[key] = value
	}
	return out
}

// userIDOf reads the account id behind a token from /auth/me.
func userIDOf(t *testing.T, resp *http.Response) uuid.UUID {
	t.Helper()
	defer resp.Body.Close()
	var out struct {
		ID uuid.UUID `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode /auth/me: %v", err)
	}
	return out.ID
}

func feedbackRows(t *testing.T, cfg *config.Config) []db.ListAppFeedbackByStatusRow {
	t.Helper()
	pool, err := db.NewPool(context.Background(), cfg)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	defer pool.Close()
	rows, err := db.New(pool).ListAppFeedbackByStatus(context.Background(), "")
	if err != nil {
		t.Fatalf("ListAppFeedbackByStatus: %v", err)
	}
	return rows
}

func TestFeedbackRequiresAuth(t *testing.T) {
	srv, _ := testServer(t)
	resp := doJSON(t, srv, http.MethodPost, "/api/v1/feedback", "", validFeedback())
	assertStatus(t, resp, http.StatusUnauthorized, "no token")
}

func TestFeedbackValidation(t *testing.T) {
	cases := []struct {
		name string
		body map[string]any
	}{
		{"unknown category", with(validFeedback(), "category", "complaint")},
		{"missing category", with(validFeedback(), "category", nil)},
		{"empty message", with(validFeedback(), "message", "")},
		{"whitespace-only message", with(validFeedback(), "message", "   \n\t ")},
		// Counted in characters: 4001 two-byte runes is over, 4000 is not.
		{"message over 4000 characters", with(validFeedback(), "message", strings.Repeat("é", 4001))},
		{"missing route", with(validFeedback(), "route", nil)},
		{"route over 500 characters", with(validFeedback(), "route", "/"+strings.Repeat("a", 500))},
		{"missing app_version", with(validFeedback(), "app_version", nil)},
		{"app_version over 64 characters", with(validFeedback(), "app_version", strings.Repeat("v", 65))},
	}
	// The endpoint allows ten requests a minute per client, and every case
	// here is a request from the same address, so each case gets its own
	// server (and so its own limiter).
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv, _ := testServer(t)
			token := signupUser(t, srv, "fb-val-"+uuid.NewString()+"@test.local", "password123")
			resp := doJSON(t, srv, http.MethodPost, "/api/v1/feedback", token, c.body)
			assertStatus(t, resp, http.StatusBadRequest, c.name)
		})
	}

	t.Run("message of exactly 4000 characters is accepted", func(t *testing.T) {
		srv, _ := testServer(t)
		token := signupUser(t, srv, "fb-max-"+uuid.NewString()+"@test.local", "password123")
		resp := doJSON(t, srv, http.MethodPost, "/api/v1/feedback", token,
			with(validFeedback(), "message", strings.Repeat("é", 4000)))
		assertStatus(t, resp, http.StatusCreated, "4000 characters")
	})
}

// The owner is the authenticated user, whatever the body says. A body
// naming another account's id is accepted -- not refused as an unknown
// field -- and the id is ignored.
func TestFeedbackStoresTheTokensUserNotTheBodys(t *testing.T) {
	srv, cfg := testServer(t)
	suffix := uuid.NewString()
	email := "fb-owner-" + suffix + "@test.local"
	token := signupUser(t, srv, email, "password123")
	otherToken := signupUser(t, srv, "fb-other-"+suffix+"@test.local", "password123")

	ownerID := userIDOf(t, doJSON(t, srv, http.MethodGet, "/api/v1/auth/me", token, nil))
	otherID := userIDOf(t, doJSON(t, srv, http.MethodGet, "/api/v1/auth/me", otherToken, nil))

	body := with(validFeedback(), "user_id", otherID.String())
	body["message"] = "  Something broke on the career page.  "
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/feedback", strings.NewReader(mustJSON(t, body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "FeedbackTest/1.0")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("post feedback: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status %d, want 201", resp.StatusCode)
	}
	id, err := uuid.Parse(idFrom(t, resp))
	if err != nil {
		t.Fatalf("201 body carries no id: %v", err)
	}

	var found *db.ListAppFeedbackByStatusRow
	for _, row := range feedbackRows(t, cfg) {
		if row.ID == id {
			found = &row
			break
		}
	}
	if found == nil {
		t.Fatalf("feedback %s not stored", id)
	}
	if found.UserID != ownerID || found.UserID == otherID {
		t.Fatalf("stored user_id %s, want the token's %s (body claimed %s)", found.UserID, ownerID, otherID)
	}
	if found.Email != email {
		t.Errorf("joined email %q, want %q", found.Email, email)
	}
	if found.Message != "Something broke on the career page." {
		t.Errorf("message %q not trimmed", found.Message)
	}
	if found.Status != "new" {
		t.Errorf("status %q, want new", found.Status)
	}
	if found.UserAgent == nil || *found.UserAgent != "FeedbackTest/1.0" {
		t.Errorf("user_agent %v, want the request header", found.UserAgent)
	}
	if found.Route != "/career/positions/abc?tab=1" || found.AppVersion != "a1b2c3d" ||
		found.Viewport == nil || *found.Viewport != "1440x900" {
		t.Errorf("context fields not stored as sent: %+v", found)
	}
}

func TestFeedbackIsRateLimited(t *testing.T) {
	srv, _ := testServer(t)
	token := signupUser(t, srv, "fb-rate-"+uuid.NewString()+"@test.local", "password123")
	for i := 0; i < 10; i++ {
		resp := doJSON(t, srv, http.MethodPost, "/api/v1/feedback", token, validFeedback())
		assertStatus(t, resp, http.StatusCreated, "within the limit")
	}
	resp := doJSON(t, srv, http.MethodPost, "/api/v1/feedback", token, validFeedback())
	assertStatus(t, resp, http.StatusTooManyRequests, "the eleventh in a minute")
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}
