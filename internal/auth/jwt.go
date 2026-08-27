package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// tokenLifetime is how long an issued token remains valid.
const tokenLifetime = 24 * time.Hour

// ErrNoSecret is returned when signing or verification is attempted with an
// empty key.
//
// config.Validate refuses an empty JWT_SECRET at startup, which is the check
// that matters. This is the same invariant asserted where the crypto actually
// happens, because HMAC-SHA256 accepts a zero-length key without complaint —
// so anything that builds an auth path outside Config (a test, a future CLI, a
// second entry point) would otherwise reproduce #36 in full silence. A local
// invariant beats a remote one for a failure with no symptom.
var ErrNoSecret = errors.New("jwt: refusing to use an empty signing secret")

// Claims is the JWT payload. RegisteredClaims provides standard fields
// (expiry, issued-at, subject); we carry the user id as Subject.
type Claims struct {
	jwt.RegisteredClaims
}

// IssueToken creates a signed JWT for the given user. This is the single
// point of token creation — password login, and any future OAuth path,
// both funnel through here.
func IssueToken(userID uuid.UUID, secret string) (string, error) {
	if secret == "" {
		return "", ErrNoSecret
	}
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenLifetime)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

// ParseToken validates a signed JWT and returns the user id from its subject.
// Used by the auth middleware.
func ParseToken(tokenString, secret string) (uuid.UUID, error) {
	if secret == "" {
		return uuid.Nil, ErrNoSecret
	}
	var claims Claims
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(t *jwt.Token) (any, error) {
		// Enforce the expected signing method — reject anything else.
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse token: %w", err)
	}
	if !token.Valid {
		return uuid.Nil, fmt.Errorf("invalid token")
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid subject in token: %w", err)
	}
	return userID, nil
}
