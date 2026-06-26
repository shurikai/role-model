//go:build integration

package generation_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/generation"
)

var stubUserID = uuid.MustParse("a0000000-0000-0000-0000-000000000001")

func TestAssembleContext(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set, skipping integration test.")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	defer pool.Close()

	svc := generation.NewService(db.New(pool), nil)

	rc, err := svc.AssembleContext(ctx, stubUserID)
	if err != nil {
		t.Fatalf("AssembleContext: %v", err)
	}

	if len(rc.Employers) == 0 {
		t.Fatal("expected at least one employer, got none")
	}

	var foundTags bool
	for _, emp := range rc.Employers {
		if len(emp.Positions) == 0 {
			t.Errorf("employer %q has no positions", emp.Name)
			continue
		}
		for _, pos := range emp.Positions {
			if len(pos.Contributions) == 0 {
				t.Errorf("position %q (employer %q) has no contributions, but was included", pos.Title, emp.Name)
				continue
			}
			for _, c := range pos.Contributions {
				if len(c.Tags) > 0 {
					foundTags = true
				}
			}
		}
	}

	// Kestrel Consulting must NOT appear — its only contributions are inactive.
	for _, emp := range rc.Employers {
		if emp.Name == "Kestrel Consulting" {
			t.Errorf("Kestrel Consulting should be excluded (only inactive contributions) but appeared")
		}
	}

	if !foundTags {
		t.Error("expected at least one contribution with tags, found none")
	}
}
