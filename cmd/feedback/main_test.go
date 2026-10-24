//go:build integration

package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/testenv"
)

func connect(t *testing.T) *pgx.Conn {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, testenv.DatabaseURL(t))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(ctx) })
	return conn
}

// freshEmail returns an address no earlier run has used. The suite does not
// clean up after itself, matching cmd/adduser's tests.
func freshEmail(prefix string) string {
	return fmt.Sprintf("%s-%d-%s@feedback.test", prefix, time.Now().UnixNano(), uuid.NewString()[:8])
}

func seedFeedback(t *testing.T, q *db.Queries, message string) (db.AppFeedback, string) {
	t.Helper()
	ctx := context.Background()
	email := freshEmail("cli")
	user, err := q.CreateUser(ctx, db.CreateUserParams{ID: uuid.New(), Email: email})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	f, err := q.CreateAppFeedback(ctx, db.CreateAppFeedbackParams{
		ID: uuid.New(), UserID: user.ID, Category: "bug", Message: message,
		Route: "/career?x=1", AppVersion: "abc1234",
	})
	if err != nil {
		t.Fatalf("CreateAppFeedback: %v", err)
	}
	return f, email
}

func TestListShowsAReportWithItsContext(t *testing.T) {
	ctx := context.Background()
	q := db.New(connect(t))
	f, email := seedFeedback(t, q, "The import dropped a job.\nSecond line.")

	var out bytes.Buffer
	if err := list(ctx, q, "new", &out); err != nil {
		t.Fatalf("list: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		f.ID.String(), email, "category: bug", "route:    /career?x=1",
		"version:  abc1234", "    The import dropped a job.", "    Second line.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("list output missing %q", want)
		}
	}
}

func TestTriageMovesAReportOutOfNew(t *testing.T) {
	ctx := context.Background()
	q := db.New(connect(t))
	f, _ := seedFeedback(t, q, "Confusing date field.")

	if err := triage(ctx, q, f.ID); err != nil {
		t.Fatalf("triage: %v", err)
	}

	var newOut, triagedOut bytes.Buffer
	if err := list(ctx, q, "new", &newOut); err != nil {
		t.Fatalf("list new: %v", err)
	}
	if err := list(ctx, q, "triaged", &triagedOut); err != nil {
		t.Fatalf("list triaged: %v", err)
	}
	if strings.Contains(newOut.String(), f.ID.String()) {
		t.Errorf("triaged report still listed under new")
	}
	if !strings.Contains(triagedOut.String(), f.ID.String()+"  [triaged]") {
		t.Errorf("triaged report not listed under triaged")
	}
}

func TestTriageRefusesAnUnknownID(t *testing.T) {
	q := db.New(connect(t))
	if err := triage(context.Background(), q, uuid.New()); err == nil {
		t.Fatal("triage of an id that does not exist should fail")
	}
}

func TestStatusFilter(t *testing.T) {
	for in, want := range map[string]string{"new": "new", "triaged": "triaged", "all": ""} {
		got, err := statusFilter(in)
		if err != nil || got != want {
			t.Errorf("statusFilter(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := statusFilter("open"); err == nil {
		t.Error("statusFilter should refuse an unknown status")
	}
}
