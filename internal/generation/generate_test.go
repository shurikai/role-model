//go:build integration

package generation_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/generation"
	"github.com/shurikai/role-model/internal/testenv"
)

func TestGenerate(t *testing.T) {
	dsn := testenv.DatabaseURL(t)
	apiKey := testenv.AnthropicAPIKey(t)

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	defer pool.Close()

	userID := integrationUserID(t, ctx, pool)
	queries := db.New(pool)
	client := generation.NewClient(apiKey)
	svc := generation.NewService(queries, client)

	signals := json.RawMessage(`{
		"required_skills": ["Go", "distributed systems", "PostgreSQL"],
		"preferred_skills": [],
		"seniority": "senior",
		"culture_signals": ["microservices", "API design", "observability"],
		"screening_summary": {
			"industry": "payments infrastructure",
			"work_arrangement": "fully remote"
		}
	}`)

	app, err := queries.CreateApplication(ctx, db.CreateApplicationParams{
		UserID:      userID,
		CompanyName: "Test Corp",
		RoleTitle:   "Senior Software Engineer",
		JdText:      strPtr("Looking for a senior Go engineer with distributed systems experience."),
		Status:      "draft",
	})
	if err != nil {
		t.Fatalf("create test application: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM resume_versions WHERE application_id = $1", app.ID)
		pool.Exec(ctx, "DELETE FROM applications WHERE id = $1", app.ID)
	})

	_, err = queries.UpdateApplicationSignals(ctx, db.UpdateApplicationSignalsParams{
		ID:        app.ID,
		UserID:    userID,
		JdSignals: &signals,
	})
	if err != nil {
		t.Fatalf("set jd_signals: %v", err)
	}

	rv, err := svc.Generate(ctx, app.ID, userID)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if rv.VersionNumber != 1 {
		t.Errorf("expected version_number 1, got %d", rv.VersionNumber)
	}
	if rv.ApplicationID != app.ID {
		t.Errorf("application_id mismatch")
	}
	if rv.StructuredOutput == nil || len(*rv.StructuredOutput) == 0 {
		t.Error("expected non-empty structured_output")
	}
	if rv.ID == uuid.Nil {
		t.Error("expected non-zero resume version ID")
	}

	var out struct {
		Education   []json.RawMessage `json:"education"`
		Credentials []json.RawMessage `json:"credentials"`
		Projects    []json.RawMessage `json:"projects"`
	}
	if err := json.Unmarshal(*rv.StructuredOutput, &out); err != nil {
		t.Fatalf("unmarshal structured_output: %v", err)
	}
	// The old assertion named one institution, which exists in the private
	// dataset only — the same hardcoding as the user id (#90). What the
	// document must actually do is carry the education this account has, so
	// the expectation is read from the account rather than named.
	var educationRows int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM education WHERE user_id = $1`, userID,
	).Scan(&educationRows); err != nil {
		t.Fatalf("count education: %v", err)
	}
	if educationRows > 0 && len(out.Education) == 0 {
		t.Errorf("this account has %d education rows, none of which reached the document", educationRows)
	}

	// These three are arrays either way. An empty list and a null are
	// different things to a renderer, and null is the one that breaks it.
	if out.Education == nil {
		t.Error("expected an education array (may be empty, but must not be null)")
	}
	if out.Credentials == nil {
		t.Error("expected credentials array (may be empty, but must not be null)")
	}
	if out.Projects == nil {
		t.Error("expected projects array (may be empty, but must not be null)")
	}
}

func strPtr(s string) *string { return &s }
