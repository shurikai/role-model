package config

import (
	"strings"
	"testing"
)

// valid returns a Config that passes, so each case below changes exactly one
// thing and the assertion cannot pass for an unrelated reason.
func valid() Config {
	return Config{
		DatabaseURL:     "postgres://u:p@localhost:5432/db",
		JWTSecret:       strings.Repeat("k", minJWTSecretLen),
		AnthropicAPIKey: "sk-ant-test",
		Port:            "8080",
		Environment:     "development",
	}
}

func TestValidateAcceptsACompleteConfig(t *testing.T) {
	if err := valid().Validate(); err != nil {
		t.Fatalf("a complete config must validate, got: %v", err)
	}
}

// #36. The failure this guards is silent: with an empty secret the server
// starts, signup and login succeed, /auth/me verifies, and every token is
// forgeable by anyone who can guess a user's UUID. Nothing anywhere reports
// it, which is why the check has to be at startup rather than at use.
func TestValidateRefusesAnEmptyJWTSecret(t *testing.T) {
	c := valid()
	c.JWTSecret = ""

	err := c.Validate()
	if err == nil {
		t.Fatal("an empty JWT_SECRET must be refused")
	}
	if !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Errorf("the error must name the variable, got: %v", err)
	}
	// The operator has to know what to do about it, not just that it is wrong.
	if !strings.Contains(err.Error(), "openssl rand") {
		t.Errorf("the error should say how to generate one, got: %v", err)
	}
}

// A short secret is not a length error any library reports — HS256 hashes the
// key to the block size, so a four-character secret works exactly as well as a
// strong one right up until someone guesses it.
func TestValidateRefusesAShortJWTSecret(t *testing.T) {
	c := valid()
	c.JWTSecret = strings.Repeat("k", minJWTSecretLen-1)

	if err := c.Validate(); err == nil {
		t.Fatalf("a %d-character secret must be refused", minJWTSecretLen-1)
	}

	c.JWTSecret = strings.Repeat("k", minJWTSecretLen)
	if err := c.Validate(); err != nil {
		t.Fatalf("exactly %d characters must be accepted, got: %v", minJWTSecretLen, err)
	}
}

func TestValidateRefusesMissingRequiredValues(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Config)
		want string
	}{
		{"no database url", func(c *Config) { c.DatabaseURL = "" }, "DATABASE_URL"},
		{"blank database url", func(c *Config) { c.DatabaseURL = "   " }, "DATABASE_URL"},
		// Silent in a different way: the server starts fine and fails on the
		// first extraction with a 401, long after whoever deployed it left.
		{"no anthropic key", func(c *Config) { c.AnthropicAPIKey = "" }, "ANTHROPIC_API_KEY"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := valid()
			tc.mut(&c)
			err := c.Validate()
			if err == nil {
				t.Fatalf("expected %s to be required", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error must name %s, got: %v", tc.want, err)
			}
		})
	}
}

// Reporting one problem at a time makes fixing three a three-restart loop.
func TestValidateReportsEveryProblemAtOnce(t *testing.T) {
	err := Config{}.Validate()
	if err == nil {
		t.Fatal("an empty config must be refused")
	}
	for _, want := range []string{"DATABASE_URL", "JWT_SECRET", "ANTHROPIC_API_KEY"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error must name %s, got: %v", want, err)
		}
	}
}

// The warn-only settings stay warn-only. CORS failing closed and the renderer
// URL being absent are both recoverable and visible at the point of use; only
// the silent-and-unrecoverable ones stop startup.
func TestValidateDoesNotRequireTheWarnOnlySettings(t *testing.T) {
	c := valid()
	c.AllowedOrigins = nil
	c.RendererURL = ""
	if err := c.Validate(); err != nil {
		t.Fatalf("CORS origins and renderer URL must not be startup-fatal, got: %v", err)
	}
}
