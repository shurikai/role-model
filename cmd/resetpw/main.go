// Command resetpw sets a user's password directly in the database.
//
// The service has no password reset flow — signup/login/me is the whole of the
// auth surface — so a forgotten local password can only be fixed by writing
// users.password_hash. This is a stopgap for single-user self-hosted operation,
// not a substitute for a real reset flow in the UI.
//
// Usage:
//
//	make reset-password EMAIL=someone@example.com
//	go run ./cmd/resetpw -email someone@example.com
//
// The password is read from stdin so it never lands in argv, the process list,
// or shell history. If NEWPASS is set in the environment it is used instead,
// which is what makes the command usable from a pipe or a script.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

// minPasswordLength mirrors the rule enforced by the signup handler, so this
// tool cannot set a password the API itself would have rejected.
const minPasswordLength = 8

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "resetpw: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	email := flag.String("email", "", "email address of the user whose password to reset")
	flag.Parse()

	if *email == "" {
		return fmt.Errorf("-email is required")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return fmt.Errorf("DATABASE_URL is not set")
	}

	password, err := readPassword()
	if err != nil {
		return err
	}
	if len(password) < minPasswordLength {
		return fmt.Errorf("password must be at least %d characters", minPasswordLength)
	}

	// DefaultCost must match internal/api/handlers.Signup, or hashes written
	// here would differ in cost from those the service writes itself.
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer conn.Close(ctx)

	// An UPDATE, deliberately: every tenant-scoped row references the existing
	// user by user_id, so inserting a fresh user would produce a working login
	// attached to an empty career history.
	tag, err := conn.Exec(ctx,
		`UPDATE users SET password_hash = $1, updated_at = now() WHERE email = $2`,
		string(hash), *email)
	if err != nil {
		return fmt.Errorf("updating password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("no user found with email %q", *email)
	}

	fmt.Fprintf(os.Stderr, "\npassword updated for %s\n", *email)
	return nil
}

// readPassword prefers NEWPASS from the environment, falling back to a prompt
// on stdin. The env var exists because a prompt needs a terminal, and this is
// often run somewhere that has none.
func readPassword() (string, error) {
	if password, ok := os.LookupEnv("NEWPASS"); ok {
		return password, nil
	}

	fmt.Fprint(os.Stderr, "New password: ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("no password on stdin (not a terminal?) — set NEWPASS instead")
	}
	return strings.TrimRight(line, "\r\n"), nil
}
