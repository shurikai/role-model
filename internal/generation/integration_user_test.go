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
// Discovered by "who has the most employers" rather than "the first user",
// because that is the question the assertions actually depend on. Packages run
// in parallel under `go test ./...`, and the api and intake suites create
// accounts of their own through signup — so picking the oldest or the first
// user would eventually pick one of those, and a brand-new account with no
// employers fails every assertion here for reasons that have nothing to do
// with generation.
//
// No career anywhere means SKIP, not fail. An empty but migrated database is
// the correct state for a CI run that has not loaded a dataset, and a test
// that cannot run is not a test that failed.
func integrationUserID(t *testing.T, ctx context.Context, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()

	var userID uuid.UUID
	err := pool.QueryRow(ctx, `
		SELECT user_id
		FROM employers
		GROUP BY user_id
		ORDER BY count(*) DESC, user_id
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
