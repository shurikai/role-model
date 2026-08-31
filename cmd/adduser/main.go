// Command adduser creates an account directly in the database: the users row
// plus the starting vocabularies, exactly the pair POST /api/v1/auth/signup
// writes, minus the HTTP and minus the open signup route.
//
// A deployed instance runs with SIGNUP_ENABLED=false. This is how you add a
// user without briefly reopening signup to the whole internet.
//
// Usage:
//
//	make add-user EMAIL=someone@example.com
//	go run ./cmd/adduser -email someone@example.com
//	docker compose exec server /usr/local/bin/adduser -email someone@example.com
//
// A password is optional. Set PASSWORD in the environment to give the account
// one (read from the environment, never a flag, so it stays out of argv, the
// process list, and shell history — same reason cmd/resetpw uses NEWPASS).
// With no PASSWORD the account is created with a null password_hash, which is
// the normal state for an OIDC-only account; a password can be set later with
// `make reset-password`.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/vocabulary"
)

// minPasswordLength mirrors the rule the signup handler enforces (and that
// cmd/resetpw mirrors in turn), so this tool cannot set a password the API
// itself would have rejected.
const minPasswordLength = 8

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "adduser: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	email := flag.String("email", "", "email address for the new account")
	flag.Parse()

	if *email == "" {
		return fmt.Errorf("-email is required")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return fmt.Errorf("DATABASE_URL is not set")
	}

	passwordHash, err := hashPassword(os.LookupEnv("PASSWORD"))
	if err != nil {
		return err
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer conn.Close(ctx)

	id, err := addUser(ctx, conn, *email, passwordHash)
	if err != nil {
		return err
	}

	// The UUID goes to stdout so a caller can capture it; everything else is
	// stderr, keeping stdout to just the id.
	fmt.Println(id)
	fmt.Fprintf(os.Stderr, "created account for %s (%s)\n", *email, id)
	if passwordHash == nil {
		fmt.Fprintf(os.Stderr,
			"no password set — this account signs in via OIDC, or set one with `make reset-password EMAIL=%s`\n",
			*email)
	}
	return nil
}

// hashPassword turns an optional PASSWORD value into an optional bcrypt hash.
// The bool is os.LookupEnv's "was it set" — an explicitly empty PASSWORD is
// still too short and is rejected rather than silently ignored.
func hashPassword(password string, set bool) (*string, error) {
	if !set {
		return nil, nil
	}
	if len(password) < minPasswordLength {
		return nil, fmt.Errorf("PASSWORD must be at least %d characters", minPasswordLength)
	}
	// DefaultCost must match internal/api/handlers.Signup and cmd/resetpw, or a
	// hash written here would differ in cost from those the service writes.
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}
	s := string(hash)
	return &s, nil
}

// addUser writes the users row and its starting vocabularies in one
// transaction — the same pair AuthHandler.Signup writes. A half-created account
// (a row with no career_levels) still logs in but produces resumes at no
// particular altitude, quietly, so the two must commit together.
func addUser(ctx context.Context, conn *pgx.Conn, email string, passwordHash *string) (uuid.UUID, error) {
	// Friendly duplicate message. The users.email UNIQUE constraint still
	// backstops a race between this check and the insert.
	if _, err := db.New(conn).GetUserByEmail(ctx, email); err == nil {
		return uuid.Nil, fmt.Errorf("an account with email %q already exists", email)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, fmt.Errorf("checking for an existing account: %w", err)
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after a successful commit

	q := db.New(tx)

	user, err := q.CreateUser(ctx, db.CreateUserParams{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: passwordHash,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("creating user: %w", err)
	}

	if err := vocabulary.Install(ctx, q, user.ID); err != nil {
		return uuid.Nil, fmt.Errorf("installing starting vocabularies: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, fmt.Errorf("commit tx: %w", err)
	}

	return user.ID, nil
}
