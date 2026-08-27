package auth_test

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
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

// #36's other half. config.Validate refuses an empty JWT_SECRET at startup;
// this pins the same invariant where the crypto happens, because HMAC-SHA256
// takes a zero-length key without complaint and the resulting token verifies
// perfectly against the same empty key. Nothing fails, which is what made the
// original bug invisible.
func TestEmptySecretIsRefusedOnBothSides(t *testing.T) {
	if _, err := auth.IssueToken(uuid.New(), ""); !errors.Is(err, auth.ErrNoSecret) {
		t.Errorf("issue with an empty secret: got %v, want ErrNoSecret", err)
	}

	// Built with a real secret, then verified with none — the shape a
	// misconfigured deployment produces.
	tok, err := auth.IssueToken(uuid.New(), "a-real-secret-value-of-some-length")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if _, err := auth.ParseToken(tok, ""); !errors.Is(err, auth.ErrNoSecret) {
		t.Errorf("parse with an empty secret: got %v, want ErrNoSecret", err)
	}
}

// The 24-hour lifetime is only a security property if it is enforced on the
// way back in. Nothing tested that it was.
func TestParseRejectsAnExpiredToken(t *testing.T) {
	const secret = "a-real-secret-value-of-some-length"
	userID := uuid.New()

	// Issued and expired in the past, which IssueToken cannot produce.
	past := time.Now().Add(-48 * time.Hour)
	claims := auth.Claims{RegisteredClaims: jwt.RegisteredClaims{
		Subject:   userID.String(),
		IssuedAt:  jwt.NewNumericDate(past),
		ExpiresAt: jwt.NewNumericDate(past.Add(24 * time.Hour)),
	}}
	expired, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign expired token: %v", err)
	}

	if _, err := auth.ParseToken(expired, secret); err == nil {
		t.Fatal("an expired token must be rejected; the signature is valid, only the clock says no")
	}

	// The same claims, still in date, must pass — otherwise the assertion
	// above could be passing for any reason at all.
	now := time.Now()
	claims.IssuedAt = jwt.NewNumericDate(now)
	claims.ExpiresAt = jwt.NewNumericDate(now.Add(time.Hour))
	live, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign live token: %v", err)
	}
	if _, err := auth.ParseToken(live, secret); err != nil {
		t.Fatalf("an unexpired token must parse, got: %v", err)
	}
}

// The classic JWT forgery: strip the signature and claim the algorithm is
// "none". jwt.go pins the method to HMAC, and this is what says so.
func TestParseRejectsTheNoneAlgorithm(t *testing.T) {
	claims := auth.Claims{RegisteredClaims: jwt.RegisteredClaims{
		Subject:   uuid.New().String(),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}}
	unsigned, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).
		SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("build alg:none token: %v", err)
	}

	if _, err := auth.ParseToken(unsigned, "a-real-secret-value-of-some-length"); err == nil {
		t.Fatal("an alg:none token must be rejected")
	}
}
