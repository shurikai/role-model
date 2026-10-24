// Command feedback reads and triages the product feedback testers send from
// inside the app (#167). There is no admin screen on purpose: reports are
// read here and turned into GitHub issues by hand.
//
// Usage:
//
//	make feedback-list
//	make feedback-triage ID=<uuid>
//	go run ./cmd/feedback list [-status new|triaged|all]
//	go run ./cmd/feedback triage <id>
//	docker compose exec server /usr/local/bin/feedback list
//
// list prints one block per report, newest first, with everything the app
// recorded about where the tester was: account, category, route, build. The
// default is -status new, the reports nobody has looked at yet. triage marks
// one as looked at.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/shurikai/role-model/internal/db"
)

const usage = `usage:
  feedback list [-status new|triaged|all]
  feedback triage <id>`

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "feedback: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New(usage)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return fmt.Errorf("DATABASE_URL is not set")
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("list", flag.ContinueOnError)
		status := fs.String("status", "new", "new, triaged, or all")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		filter, err := statusFilter(*status)
		if err != nil {
			return err
		}
		return withQueries(databaseURL, func(ctx context.Context, q *db.Queries) error {
			return list(ctx, q, filter, out)
		})
	case "triage":
		if len(args) != 2 {
			return errors.New(usage)
		}
		id, err := uuid.Parse(args[1])
		if err != nil {
			return fmt.Errorf("%q is not a feedback id", args[1])
		}
		return withQueries(databaseURL, func(ctx context.Context, q *db.Queries) error {
			if err := triage(ctx, q, id); err != nil {
				return err
			}
			fmt.Fprintf(out, "marked %s triaged\n", id)
			return nil
		})
	default:
		return fmt.Errorf("unknown command %q\n%s", args[0], usage)
	}
}

// statusFilter maps the -status flag onto the query's argument, where an
// empty string means every status.
func statusFilter(status string) (string, error) {
	switch status {
	case "new", "triaged":
		return status, nil
	case "all":
		return "", nil
	default:
		return "", fmt.Errorf("-status must be new, triaged, or all, not %q", status)
	}
}

func withQueries(databaseURL string, fn func(context.Context, *db.Queries) error) error {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer conn.Close(ctx)
	return fn(ctx, db.New(conn))
}

func list(ctx context.Context, q *db.Queries, status string, out io.Writer) error {
	rows, err := q.ListAppFeedbackByStatus(ctx, status)
	if err != nil {
		return fmt.Errorf("listing feedback: %w", err)
	}
	if len(rows) == 0 {
		fmt.Fprintln(out, "no feedback")
		return nil
	}
	for i, row := range rows {
		if i > 0 {
			fmt.Fprintln(out)
		}
		writeBlock(out, row)
	}
	return nil
}

func writeBlock(out io.Writer, row db.ListAppFeedbackByStatusRow) {
	fmt.Fprintf(out, "%s  [%s]\n", row.ID, row.Status)
	fmt.Fprintf(out, "  created:  %s\n", row.CreatedAt.Local().Format("2006-01-02 15:04 MST"))
	fmt.Fprintf(out, "  from:     %s\n", row.Email)
	fmt.Fprintf(out, "  category: %s\n", row.Category)
	fmt.Fprintf(out, "  route:    %s\n", row.Route)
	fmt.Fprintf(out, "  version:  %s\n", row.AppVersion)
	fmt.Fprintln(out)
	for _, line := range strings.Split(row.Message, "\n") {
		fmt.Fprintf(out, "    %s\n", line)
	}
}

func triage(ctx context.Context, q *db.Queries, id uuid.UUID) error {
	n, err := q.MarkAppFeedbackTriaged(ctx, id)
	if err != nil {
		return fmt.Errorf("marking %s triaged: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("no feedback with id %s", id)
	}
	return nil
}
