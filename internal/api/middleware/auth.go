package middleware

import (
	"net/http"
	"strings"

	"github.com/shurikai/role-model/internal/auth"
	"github.com/shurikai/role-model/internal/httputil"
)

// RequireAuth validates the bearer token and stashes the user id in the request
// context. Requests without a valid token are rejected with 401.
func RequireAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				httputil.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing authorization header")
				return
			}

			// Expect "Bearer <token>".
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				httputil.WriteError(w, http.StatusUnauthorized, "unauthorized", "malformed authorization header")
				return
			}

			userID, err := auth.ParseToken(parts[1], secret)
			if err != nil {
				httputil.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
				return
			}

			ctx := httputil.WithUserID(r.Context(), userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
