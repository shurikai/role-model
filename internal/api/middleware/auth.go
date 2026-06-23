package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/shurikai/role-model/internal/auth"
)

// contextKey is unexported so no other package can collide with our context value.
type contextKey string

const userIDKey contextKey = "userID"

// RequireAuth validates the bearer token and stashes the user id in the request
// context. Requests without a valid token are rejected with 401.
func RequireAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeError(w, http.StatusUnauthorized, "unauthorized", "missing authorization header")
				return
			}

			// Expect "Bearer <token>".
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				writeError(w, http.StatusUnauthorized, "unauthorized", "malformed authorization header")
				return
			}

			userID, err := auth.ParseToken(parts[1], secret)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromContext retrieves the authenticated user id stashed by RequireAuth.
// The bool is false if no user id is present (i.e. the handler ran outside the
// auth middleware — a programming error, not a runtime condition).
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDKey).(uuid.UUID)
	return id, ok
}
