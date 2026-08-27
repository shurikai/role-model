package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/shurikai/role-model/internal/httputil"
)

// MaxRequestBody caps every decoded request body.
//
// There was no cap at all. decodeJSON is the universal decoder, so an
// authenticated caller could POST an arbitrarily large body and the server
// would buffer it — and on the import routes the body is not merely buffered,
// it is sent to a model as the user message. Input tokens are the expensive
// half of an LLM call, so an unbounded body here is a bill as well as a
// memory problem.
//
// 2 MB is roughly 500k tokens of prose, far past any real résumé or job
// description and far short of anything that threatens the process. The limit
// exists to make the pathological case impossible, not to police the ordinary
// one.
const MaxRequestBody = 2 << 20

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	// MaxBytesReader also caps what the server will read off the socket, so an
	// oversized body is refused as it arrives rather than after it has all
	// been buffered.
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBody)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		// A body over the cap is a distinct answer from malformed JSON: 413
		// tells the caller to send less, 400 tells them to fix their syntax,
		// and reporting the first as the second sends them looking for a
		// quoting bug that is not there.
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			httputil.WriteError(w, http.StatusRequestEntityTooLarge, "body_too_large",
				fmt.Sprintf("request body exceeds the %d byte limit", MaxRequestBody))
			return false
		}
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
