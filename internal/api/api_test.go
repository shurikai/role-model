//go:build integration

package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/shurikai/role-model/internal/api"
	"github.com/shurikai/role-model/internal/config"
	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/generation"
)

// testServer spins up the real router against the test database and returns
// an httptest.Server. Skips the whole test if env isn't configured.
func testServer(t *testing.T) (*httptest.Server, *config.Config) {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	secret := os.Getenv("JWT_SECRET")
	if dsn == "" || secret == "" {
		t.Skip("DATABASE_URL and JWT_SECRET required for API integration tests")
	}

	cfg := &config.Config{
		DatabaseURL: dsn,
		JWTSecret:   secret,
		Environment: "test",
	}

	pool, err := db.NewPool(context.Background(), cfg)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	t.Cleanup(pool.Close)

	queries := db.New(pool)
	genClient := generation.NewClient(cfg.AnthropicAPIKey) // not exercised in these tests
	genSvc := generation.NewService(queries, genClient)    // however your Service constructor is shaped

	router := api.NewRouter(api.RouterDeps{
		Pool:      pool,
		Queries:   queries,
		GenSvc:    genSvc,
		JWTSecret: cfg.JWTSecret,
	})
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	return srv, cfg
}

// signupUser creates a user and returns its auth token.
func signupUser(t *testing.T, srv *httptest.Server, email, password string) string {
	t.Helper()
	body := map[string]string{"email": email, "password": password}
	resp := doJSON(t, srv, http.MethodPost, "/api/v1/auth/signup", "", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("signup %s: status %d", email, resp.StatusCode)
	}
	var out struct {
		Token string `json:"token"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	return out.Token
}

// doJSON performs a request with optional bearer token and JSON body.
func doJSON(t *testing.T, srv *httptest.Server, method, path, token string, body any) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req, err := http.NewRequest(method, srv.URL+path, &buf)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

// idFrom extracts the "id" field from a JSON response body.
func idFrom(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	var out struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode id: %v", err)
	}
	return out.ID
}

func TestMultiTenantIsolation(t *testing.T) {
	srv, _ := testServer(t)

	// Two distinct users. Unique emails so reruns don't collide.
	suffix := uuid.NewString()
	tokenA := signupUser(t, srv, "a-"+suffix+"@test.local", "passwordA1")
	tokenB := signupUser(t, srv, "b-"+suffix+"@test.local", "passwordB1")

	// User B creates an employer.
	resp := doJSON(t, srv, http.MethodPost, "/api/v1/employers", tokenB,
		map[string]string{"name": "B's Corp"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("B create employer: status %d", resp.StatusCode)
	}
	bEmployerID := idFrom(t, resp)

	// User A must NOT be able to read B's employer.
	resp = doJSON(t, srv, http.MethodGet, "/api/v1/employers/"+bEmployerID, tokenA, nil)
	assertStatus(t, resp, http.StatusNotFound, "A reads B's employer")

	// User A must NOT be able to update B's employer.
	resp = doJSON(t, srv, http.MethodPatch, "/api/v1/employers/"+bEmployerID, tokenA,
		map[string]string{"name": "Hijacked"})
	assertStatus(t, resp, http.StatusNotFound, "A updates B's employer")

	// User A must NOT be able to create a position under B's employer.
	resp = doJSON(t, srv, http.MethodPost, "/api/v1/positions", tokenA, map[string]any{
		"employer_id": bEmployerID,
		"title":       "Intruder",
		"started_on":  "2020-01-01",
		"sort_order":  0,
	})
	assertStatus(t, resp, http.StatusNotFound, "A creates position under B's employer")
}

func assertStatus(t *testing.T, resp *http.Response, want int, label string) {
	t.Helper()
	defer resp.Body.Close()
	if resp.StatusCode != want {
		t.Errorf("%s: got status %d, want %d", label, resp.StatusCode, want)
	}
}
