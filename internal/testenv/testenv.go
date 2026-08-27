// Package testenv centralises how integration tests find their environment.
//
// # Why this exists
//
// Every integration suite in this repository begins by reading DATABASE_URL
// and calling t.Skip when it is absent. That is right for a developer who has
// not started Postgres, and catastrophic in CI: a job that forgets an
// environment variable reports a green `ok` for a package in which nothing
// ran. #99 is the case where 24 test functions had never executed at all; the
// subtler half of it is that wiring them up wrong looks exactly like wiring
// them up right.
//
// So the skip is conditional. Set REQUIRE_INTEGRATION=1 — as CI does — and a
// missing variable becomes a failure that names what is missing, instead of a
// silence that reads as success.
//
// The logic lived in thirteen copies across seven files before this, which is
// why it was possible for them to disagree about what they needed.
package testenv

import (
	"os"
	"testing"
)

// requireEnvVar is the name of the switch that turns skips into failures.
const requireEnvVar = "REQUIRE_INTEGRATION"

// Required reports whether integration tests must run rather than skip.
func Required() bool {
	return os.Getenv(requireEnvVar) != ""
}

// missing ends the test, as a failure under REQUIRE_INTEGRATION and as a skip
// otherwise.
func missing(t *testing.T, what, why string) {
	t.Helper()
	if Required() {
		t.Fatalf("%s is not set, but %s=1 says integration tests must run: %s",
			what, requireEnvVar, why)
	}
	t.Skipf("%s not set, skipping: %s", what, why)
}

// DatabaseURL returns the DSN for the test database.
func DatabaseURL(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		missing(t, "DATABASE_URL", "these tests run against a live, migrated database")
	}
	return dsn
}

// JWTSecret returns the signing secret for tests that mint tokens.
//
// Separate from DatabaseURL because the API suite needs both and the others
// need only the database — reporting the one that is actually missing beats
// reporting the pair.
func JWTSecret(t *testing.T) string {
	t.Helper()
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		missing(t, "JWT_SECRET", "these tests sign and verify real tokens")
	}
	return secret
}

// AnthropicAPIKey returns the key for the few tests that call a model.
//
// These stay skippable even under REQUIRE_INTEGRATION: they cost money and
// return a different answer each time, so a CI run that skips them is correct
// rather than under-tested. Nothing else in the suite behaves this way.
func AnthropicAPIKey(t *testing.T) string {
	t.Helper()
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		t.Skip("ANTHROPIC_API_KEY not set, skipping: this test spends money on a real model call")
	}
	return key
}
