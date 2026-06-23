package auth_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/shurikai/role-model/internal/auth"
)

func TestTokenRoundTrip(t *testing.T) {
	secret := "test-secret"
	userID := uuid.New()

	tok, err := auth.IssueToken(userID, secret)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	got, err := auth.ParseToken(tok, secret)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got != userID {
		t.Fatalf("round trip mismatch: got %s want %s", got, userID)
	}
}

func TestParseRejectsWrongSecret(t *testing.T) {
	tok, _ := auth.IssueToken(uuid.New(), "secret-a")
	if _, err := auth.ParseToken(tok, "secret-b"); err == nil {
		t.Fatal("expected error parsing with wrong secret, got nil")
	}
}
