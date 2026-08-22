//go:build integration

package generation_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/generation"
)

func TestGenerate(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	defer pool.Close()

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
		UserID:      stubUserID,
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
		UserID:    stubUserID,
		JdSignals: &signals,
	})
	if err != nil {
		t.Fatalf("set jd_signals: %v", err)
	}

	rv, err := svc.Generate(ctx, app.ID, stubUserID)
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
	if len(out.Education) == 0 {
		t.Error("expected education to be populated (Tulane is seeded)")
	}
	if out.Credentials == nil {
		t.Error("expected credentials array (may be empty, but must not be null)")
	}
	if out.Projects == nil {
		t.Error("expected projects array (may be empty, but must not be null)")
	}
}

func strPtr(s string) *string { return &s }
