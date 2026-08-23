// Command intakerun drives a career import end to end against a real model:
// pasted text in, staged drafts out, resolved into rows.
//
// It exists because Phase 9's acceptance test is "build a second career THROUGH
// the intake", and a hand-authored second seed proves the pipeline is neutral
// while proving nothing about the low-friction path. This is the harness that
// makes that runnable and repeatable rather than a one-off session artifact.
//
// It is deliberately not wired into the server. It spends money, it writes a
// whole career, and both of those want an explicit invocation.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/generation"
	"github.com/shurikai/role-model/internal/intake"
	"github.com/shurikai/role-model/internal/vocabulary"
)

func main() {
	var (
		file   = flag.String("file", "", "path to the career text to import")
		email  = flag.String("email", "", "email for the account to create")
		userID = flag.String("user", "", "optional fixed UUID for the account")
		dryRun = flag.Bool("dry-run", false, "stage drafts but do not resolve them")
	)
	flag.Parse()

	if *file == "" || *email == "" {
		log.Fatal("both -file and -email are required")
	}
	dsn := os.Getenv("DATABASE_URL")
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if dsn == "" || apiKey == "" {
		log.Fatal("DATABASE_URL and ANTHROPIC_API_KEY are both required")
	}

	text, err := os.ReadFile(*file)
	if err != nil {
		log.Fatalf("read %s: %v", *file, err)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	q := db.New(pool)
	svc := intake.NewService(pool, q)

	uid := uuid.New()
	if *userID != "" {
		if uid, err = uuid.Parse(*userID); err != nil {
			log.Fatalf("parse -user: %v", err)
		}
	}

	// The account starts exactly as a signup leaves it: a user row and the
	// shipped neutral vocabularies, nothing else. That is the whole point —
	// no employers, no positions, no tags, no categories.
	if _, err := q.CreateUser(ctx, db.CreateUserParams{ID: uid, Email: *email}); err != nil {
		log.Fatalf("create user: %v", err)
	}
	if err := vocabulary.Install(ctx, q, uid); err != nil {
		log.Fatalf("install vocabulary: %v", err)
	}
	fmt.Printf("account %s created with the neutral default vocabularies\n", uid)

	batch, err := q.CreateImportBatch(ctx, db.CreateImportBatchParams{
		ID: uuid.New(), UserID: uid, RawText: string(text), Status: "extracting",
	})
	if err != nil {
		log.Fatalf("create batch: %v", err)
	}

	drafts, err := svc.ExtractCareer(ctx, generation.NewClient(apiKey), uid, batch.ID, string(text))
	if err != nil {
		log.Fatalf("extract career: %v", err)
	}

	counts := map[string]int{}
	flagged := 0
	for _, d := range drafts {
		counts[d.Kind]++
		if d.Flags != nil {
			flagged++
			fmt.Printf("  FLAG %s %s: %s\n", d.Kind, d.ID, string(*d.Flags))
		}
	}
	out, _ := json.Marshal(counts)
	fmt.Printf("staged %d drafts %s (%d flagged for review)\n", len(drafts), out, flagged)

	if *dryRun {
		fmt.Println("dry run: drafts staged, nothing resolved")
		return
	}

	result, err := svc.ResolveBatch(ctx, uid, batch.ID)
	if err != nil {
		for id, why := range result.Unresolved {
			fmt.Printf("  UNRESOLVED %s: %s\n", id, why)
		}
		log.Fatalf("resolve batch: %v", err)
	}
	fmt.Printf("resolved %d drafts into rows\n", len(result.Resolved))

	if _, err := q.UpdateImportBatchStatus(ctx, db.UpdateImportBatchStatusParams{
		ID: batch.ID, UserID: uid, Status: "complete",
	}); err != nil {
		log.Printf("warning: could not mark batch complete: %v", err)
	}
}
