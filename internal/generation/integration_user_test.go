//go:build integration

package generation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// integrationUserID finds an account with a career in it.
//
// These tests used to hardcode a0000000-…, which is the PRIVATE seed's user.
// `database/sample` seeds 5a000000-… and `database/sample-clinical` seeds
// 5b000000-…, so anyone who cloned this repository and ran the integration
// suite against either public dataset got a failure they had no way to fix,
// and the natural reading was that the repo was broken rather than the test
// (#90). A hardcoded id also cannot survive a fourth dataset, and the whole
// point of the public datasets is that there will be more of them.
//
// Discovered by "who has the most ASSEMBLABLE employers" rather than "who has
// the most employers", and the distinction is the whole correctness of this
// helper. AssembleContext drops a position with no active contributions and
// then an employer left with no positions, so an account can hold employers
// and assemble to nothing at all.
//
// Counting plain employers made this flaky rather than wrong-looking. The api
// suite creates three employers per test account -- exactly tying the sample
// dataset's three -- and the tiebreak is user_id ascending, so any random test
// UUID sorting below the sample user's 5a000000-... won the tie. That account
// then assembles to nothing, because TestContributionDelete deletes the only
// contribution it made. Roughly a one-in-three coin flip per test account, and
// several accounts per run; it passed locally and failed in CI on identical
// code.
//
// No career anywhere means SKIP, not fail. An empty but migrated database is
// the correct state for a CI run that has not loaded a dataset, and a test
// that cannot run is not a test that failed.
func integrationUserID(t *testing.T, ctx context.Context, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()

	var userID uuid.UUID
	// EXISTS rather than a join, so an employer with several active
	// contributions still counts once -- the ranking is over employers, which
	// is what the assertions below are about.
	err := pool.QueryRow(ctx, `
		SELECT e.user_id
		FROM employers e
		WHERE EXISTS (
			SELECT 1
			FROM positions p
			JOIN contributions c ON c.position_id = p.id
			WHERE p.employer_id = e.id
			  AND c.is_active
		)
		GROUP BY e.user_id
		ORDER BY count(*) DESC, e.user_id
		LIMIT 1
	`).Scan(&userID)

	if errors.Is(err, pgx.ErrNoRows) {
		t.Skip("no seeded career in this database (`make seed` or `make seed-sample`), skipping")
	}
	if err != nil {
		t.Fatalf("find a seeded user: %v", err)
	}
	return userID
}
