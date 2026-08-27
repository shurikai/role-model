//go:build integration

package generation_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/generation"
	"github.com/shurikai/role-model/internal/testenv"
)

func TestAssembleContext(t *testing.T) {
	dsn := testenv.DatabaseURL(t)

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	defer pool.Close()

	userID := integrationUserID(t, ctx, pool)
	svc := generation.NewService(db.New(pool), nil)

	rc, err := svc.AssembleContext(ctx, userID)
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

	assertActiveContributionRule(t, ctx, pool, userID, rc)

	if !foundTags {
		t.Error("expected at least one contribution with tags, found none")
	}
}

// assertActiveContributionRule checks both directions of AssembleContext's
// inclusion rule against what the database actually holds.
//
// This used to be one line naming a specific employer: "X must NOT appear —
// its only contributions are inactive." That employer exists only in the
// private seed, so against either public dataset the check passed by finding
// nothing by that name — reporting success for a rule it had not tested. A
// hardcoded company name is the same defect as a hardcoded user id (#90),
// one layer down and harder to notice, because it fails open.
//
// Derived from the data instead: whichever employers have no active
// contribution behind them must be absent, and whichever do must be present.
// That holds for any dataset, and it is a stronger assertion than the name
// was — it checks every employer rather than one.
func assertActiveContributionRule(
	t *testing.T, ctx context.Context, pool *pgxpool.Pool,
	userID any, rc *generation.ResumeContext,
) {
	t.Helper()

	present := map[string]bool{}
	for _, emp := range rc.Employers {
		present[emp.Name] = true
	}

	// An employer reaches the resume only through a position holding at least
	// one active contribution — see the two `continue`s at the end of
	// AssembleContext.
	rows, err := pool.Query(ctx, `
		SELECT e.name,
		       EXISTS (
		           SELECT 1
		           FROM positions p
		           JOIN contributions c
		             ON c.position_id = p.id AND c.user_id = p.user_id
		           WHERE p.employer_id = e.id
		             AND p.user_id = e.user_id
		             AND c.is_active
		       ) AS has_active
		FROM employers e
		WHERE e.user_id = $1
	`, userID)
	if err != nil {
		t.Fatalf("query employers: %v", err)
	}
	defer rows.Close()

	var excluded int
	for rows.Next() {
		var name string
		var hasActive bool
		if err := rows.Scan(&name, &hasActive); err != nil {
			t.Fatalf("scan employer: %v", err)
		}
		switch {
		case hasActive && !present[name]:
			t.Errorf("employer %q has active contributions but was excluded", name)
		case !hasActive && present[name]:
			t.Errorf("employer %q has no active contributions but appeared", name)
		case !hasActive:
			excluded++
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate employers: %v", err)
	}

	// Said out loud rather than passing quietly. A dataset where every
	// employer has active work exercises only half this rule, and a reader
	// looking at a green run deserves to know which half ran.
	if excluded == 0 {
		t.Log("no employer in this dataset lacks active contributions; " +
			"the exclusion half of the rule was not exercised")
	}
}
