package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/shurikai/role-model/internal/httputil"
)

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_body", "request body is not valid JSON: "+err.Error())
		return false
	}
	return true
}

// isUniqueViolation reports whether err is Postgres' 23505.
//
// Several tables carry uniqueness that is a real user-facing rule rather than
// an internal invariant — skills is UNIQUE (user_id, tag_id) because a person
// has one depth for a thing, and preferences is
// UNIQUE (user_id, preference_type, label). Letting either surface as a 500
// tells the caller the server broke when in fact they asked for something the
// data model does not allow, and gives them nothing to act on.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
