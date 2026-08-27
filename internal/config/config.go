package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL     string
	AnthropicAPIKey string
	Port            string
	Environment     string
	JWTSecret       string
	AllowedOrigins  []string
	RendererURL     string
	// SignupEnabled controls whether /auth/signup accepts new accounts.
	// Defaults to TRUE in development and FALSE anywhere else — see Load.
	SignupEnabled bool
}

func Load() Config {
	_ = godotenv.Load() // loads .env into the environment if present; silent if absent

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = "development"
	}

	allowedOrigins := parseAllowedOrigins(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if len(allowedOrigins) == 0 {
		if env == "development" {
			allowedOrigins = []string{"http://localhost:5173"}
		} else {
			log.Println("WARNING: CORS_ALLOWED_ORIGINS is not set; cross-origin requests will be rejected")
		}
	}

	rendererURL := os.Getenv("RENDERER_URL")
	if rendererURL == "" {
		if env == "development" {
			rendererURL = "http://localhost:8000"
		} else {
			log.Println("WARNING: RENDERER_URL is not set; resume rendering will fail")
		}
	}

	// Signup gating. An open signup route on an internet-reachable instance
	// lets anyone create an account and spend the operator's Anthropic key,
	// and this is single-user software by default — the second account is the
	// unusual case, not the first.
	//
	// The default flips on ENVIRONMENT for the same reason CORS and the
	// renderer URL do: a developer running locally should not have to set a
	// variable to create their own account, and a deployment should not have
	// to remember to close a door it never knew was open. Set
	// SIGNUP_ENABLED=true explicitly to run an instance that accepts new
	// accounts.
	signupEnabled := env == "development"
	if raw := os.Getenv("SIGNUP_ENABLED"); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			log.Printf("WARNING: SIGNUP_ENABLED=%q is not a boolean; treating signup as %v", raw, signupEnabled)
		} else {
			signupEnabled = parsed
		}
	}

	return Config{
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		AnthropicAPIKey: os.Getenv("ANTHROPIC_API_KEY"),
		Port:            port,
		Environment:     env,
		JWTSecret:       os.Getenv("JWT_SECRET"),
		AllowedOrigins:  allowedOrigins,
		RendererURL:     rendererURL,
		SignupEnabled:   signupEnabled,
	}
}

// minJWTSecretLen is the shortest secret Validate accepts.
//
// HS256 keys are hashed to the block size, so a short one is not a length
// error the library will report — it is simply weak, silently. 32 bytes is
// what `openssl rand -base64 32` produces, which is what .env.example tells
// the operator to run.
const minJWTSecretLen = 32

// Validate reports the misconfigurations that must stop the process.
//
// The bar for being here is that the failure is SILENT: the server starts,
// serves traffic, and does the wrong thing with no error anyone would notice.
// Anything that fails loudly at the point of use does not belong.
//
// JWT_SECRET is the reason this function exists (#36). It had no default, no
// warning and no check: an empty value flows to auth.IssueToken, HMAC-SHA256
// accepts a zero-length key, and every token the deployment issues is
// forgeable by anyone who can guess a user's UUID. Signup succeeds, login
// succeeds, /auth/me verifies. There is no symptom at all.
//
// Every OTHER optional setting in Load already defaults or warns; the one
// where empty is a vulnerability was the one with no guard.
func (c Config) Validate() error {
	var problems []string

	if strings.TrimSpace(c.DatabaseURL) == "" {
		problems = append(problems,
			"DATABASE_URL is not set")
	}
	switch {
	case c.JWTSecret == "":
		problems = append(problems,
			"JWT_SECRET is not set: an empty secret signs every token with a "+
				"zero-length key, which makes them forgeable. Generate one with "+
				"`openssl rand -base64 32`")
	case len(c.JWTSecret) < minJWTSecretLen:
		problems = append(problems,
			fmt.Sprintf("JWT_SECRET is %d characters; use at least %d "+
				"(`openssl rand -base64 32`)", len(c.JWTSecret), minJWTSecretLen))
	}
	if strings.TrimSpace(c.AnthropicAPIKey) == "" {
		problems = append(problems,
			"ANTHROPIC_API_KEY is not set: the server would start and then fail "+
				"on the first extraction with a 401 from the API. You supply your "+
				"own key; this service never ships one")
	}

	if len(problems) == 0 {
		return nil
	}
	return errors.New("invalid configuration:\n  - " + strings.Join(problems, "\n  - "))
}

func parseAllowedOrigins(raw string) []string {
	if raw == "" {
		return nil
	}
	var origins []string
	for _, origin := range strings.Split(raw, ",") {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			origins = append(origins, origin)
		}
	}
	return origins
}
