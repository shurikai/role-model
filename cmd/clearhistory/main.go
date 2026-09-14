// Command clearhistory deletes a user's career and application history —
// employers, positions, contributions, tags, skills, education, credentials,
// preferences, projects, applications, resume versions, fit reports, and any
// pending import batches — while leaving the account itself untouched: the
// users row, its password, and the starting career_levels/
// proficiency_levels/resume_sections vocabulary rows all survive.
//
// Built for resetting a test account between manual passes (repeatedly
// driving the #117 onboarding interview end to end, say) without
// recreating the account or reseeding its vocabulary each time.
//
// Usage:
//
//	make clear-history EMAIL=someone@example.com          # dry run
//	make clear-history EMAIL=someone@example.com CONFIRM=1
//	go run ./cmd/clearhistory -email someone@example.com -yes
//
// -yes is required to actually delete anything. Without it, every DELETE
// still runs — so the row counts printed are real, not guessed — but the
// transaction is rolled back instead of committed, the same
// compute-then-discard pattern intake.StageDrafts uses a rollback for
// elsewhere in this codebase, just used here for a dry run instead of an
// error path.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "clearhistory: %v\n", err)
		os.Exit(1)
	}
}

// clears is every DELETE this command runs, in FK-safe order (children
// before parents — every FK in this schema is restrict-by-default, so the
// wrong order fails loudly rather than corrupting anything). Join tables
// (contribution_tags, project_contributions, project_tags, education_tags,
// credential_tags) carry no user_id column of their own and are scoped
// through their parent instead.
var clears = []struct {
	table string
	sql   string
}{
	{"contribution_feedback", `DELETE FROM contribution_feedback WHERE user_id = $1`},
	{"fit_reports", `DELETE FROM fit_reports WHERE user_id = $1`},
	{"resume_versions", `DELETE FROM resume_versions WHERE user_id = $1`},
	{"applications", `DELETE FROM applications WHERE user_id = $1`},
	{"entity_drafts", `DELETE FROM entity_drafts WHERE user_id = $1`},
	{"contribution_drafts", `DELETE FROM contribution_drafts WHERE user_id = $1`},
	{"import_batches", `DELETE FROM import_batches WHERE user_id = $1`},
	{"contribution_tags", `DELETE FROM contribution_tags WHERE contribution_id IN (SELECT id FROM contributions WHERE user_id = $1)`},
	{"project_contributions", `DELETE FROM project_contributions WHERE project_id IN (SELECT id FROM projects WHERE user_id = $1)`},
	{"project_tags", `DELETE FROM project_tags WHERE project_id IN (SELECT id FROM projects WHERE user_id = $1)`},
	{"education_tags", `DELETE FROM education_tags WHERE education_id IN (SELECT id FROM education WHERE user_id = $1)`},
	{"credential_tags", `DELETE FROM credential_tags WHERE credential_id IN (SELECT id FROM credentials WHERE user_id = $1)`},
	{"skills", `DELETE FROM skills WHERE user_id = $1`},
	{"contributions", `DELETE FROM contributions WHERE user_id = $1`},
	{"projects", `DELETE FROM projects WHERE user_id = $1`},
	{"positions", `DELETE FROM positions WHERE user_id = $1`},
	{"employers", `DELETE FROM employers WHERE user_id = $1`},
	{"education", `DELETE FROM education WHERE user_id = $1`},
	{"credentials", `DELETE FROM credentials WHERE user_id = $1`},
	{"preferences", `DELETE FROM preferences WHERE user_id = $1`},
	{"tags", `DELETE FROM tags WHERE user_id = $1`},
	{"tag_categories", `DELETE FROM tag_categories WHERE user_id = $1`},
}

func run() error {
	email := flag.String("email", "", "email address of the account to clear")
	confirm := flag.Bool("yes", false, "actually delete -- without this, reports counts and rolls back")
	flag.Parse()

	if *email == "" {
		return fmt.Errorf("-email is required")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return fmt.Errorf("DATABASE_URL is not set")
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer conn.Close(ctx)

	var userID string
	if err := conn.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, *email).Scan(&userID); err != nil {
		return fmt.Errorf("no user found with email %q: %w", *email, err)
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after a successful commit; IS the dry-run mechanism otherwise

	fmt.Fprintf(os.Stderr, "%s (%s):\n", *email, userID)
	total := int64(0)
	for _, c := range clears {
		tag, err := tx.Exec(ctx, c.sql, userID)
		if err != nil {
			return fmt.Errorf("deleting %s: %w", c.table, err)
		}
		n := tag.RowsAffected()
		total += n
		if n > 0 {
			fmt.Fprintf(os.Stderr, "  %-22s %d\n", c.table, n)
		}
	}

	if !*confirm {
		fmt.Fprintf(os.Stderr, "\n%d rows would be deleted. Dry run only -- re-run with -yes to actually delete.\n", total)
		return nil
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing: %w", err)
	}
	fmt.Fprintf(os.Stderr, "\n%d rows deleted. The account, its password, and its starting vocabulary are untouched.\n", total)
	return nil
}
