//go:build integration

package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

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
// clean up after itself, matching the internal/api integration tests.
func freshEmail(prefix string) string {
	return fmt.Sprintf("%s-%d-%s@adduser.test", prefix, time.Now().UnixNano(), uuid.NewString()[:8])
}

func TestAddUserCreatesAccountAndVocabularies(t *testing.T) {
	ctx := context.Background()
	conn := connect(t)
	q := db.New(conn)
	email := freshEmail("vocab")

	id, err := addUser(ctx, conn, email, nil)
	if err != nil {
		t.Fatalf("addUser: %v", err)
	}

	user, err := q.GetUserByEmail(ctx, email)
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if user.ID != id {
		t.Fatalf("returned id %s, stored id %s", id, user.ID)
	}
	if user.PasswordHash != nil {
		t.Errorf("password_hash should be NULL when no password was given, got %q", *user.PasswordHash)
	}

	levels, err := q.ListCareerLevelsByUser(ctx, id)
	if err != nil {
		t.Fatalf("ListCareerLevelsByUser: %v", err)
	}
	if len(levels) != 3 {
		t.Errorf("career_levels = %d, want 3 (vocabulary.Install did not run in the committed tx)", len(levels))
	}
	prof, err := q.ListProficiencyLevelsByUser(ctx, id)
	if err != nil {
		t.Fatalf("ListProficiencyLevelsByUser: %v", err)
	}
	if len(prof) != 3 {
		t.Errorf("proficiency_levels = %d, want 3", len(prof))
	}
	sections, err := q.ListResumeSectionsByUser(ctx, id)
	if err != nil {
		t.Fatalf("ListResumeSectionsByUser: %v", err)
	}
	if len(sections) != 6 {
		t.Errorf("resume_sections = %d, want 6", len(sections))
	}
}

func TestAddUserStoresPassword(t *testing.T) {
	ctx := context.Background()
	conn := connect(t)
	q := db.New(conn)
	email := freshEmail("pw")
	const plaintext = "correcthorsebattery"

	hash, err := hashPassword(plaintext, true)
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	if hash == nil {
		t.Fatal("hashPassword returned nil for a valid password")
	}

	if _, err := addUser(ctx, conn, email, hash); err != nil {
		t.Fatalf("addUser: %v", err)
	}

	user, err := q.GetUserByEmail(ctx, email)
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if user.PasswordHash == nil {
		t.Fatal("password_hash is NULL, want a stored hash")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(plaintext)); err != nil {
		t.Errorf("stored hash does not verify against the plaintext: %v", err)
	}
}

func TestAddUserRejectsDuplicate(t *testing.T) {
	ctx := context.Background()
	conn := connect(t)
	email := freshEmail("dup")

	if _, err := addUser(ctx, conn, email, nil); err != nil {
		t.Fatalf("first addUser: %v", err)
	}
	if _, err := addUser(ctx, conn, email, nil); err == nil {
		t.Fatal("second addUser with the same email returned no error")
	}

	var n int
	if err := conn.QueryRow(ctx, "select count(*) from users where email = $1", email).Scan(&n); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if n != 1 {
		t.Fatalf("want exactly 1 user row for %s, got %d", email, n)
	}
}

func TestHashPassword(t *testing.T) {
	if h, err := hashPassword("", false); err != nil || h != nil {
		t.Errorf("unset PASSWORD: got (%v, %v), want (nil, nil)", h, err)
	}
	if _, err := hashPassword("short", true); err == nil {
		t.Error("a 5-character PASSWORD was accepted")
	}
	if h, err := hashPassword("longenough1", true); err != nil || h == nil {
		t.Errorf("valid PASSWORD: got (%v, %v), want (hash, nil)", h, err)
	}
}
