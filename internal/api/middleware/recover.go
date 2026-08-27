package middleware

import (
	"log"
	"net/http"
	"runtime/debug"

	"github.com/shurikai/role-model/internal/httputil"
)

// Recoverer turns a panic into a 500 and writes the panic value and stack to
// the log.
//
// It logged neither until now. chi's own middleware.Recoverer does; this one
// replaced it to get a structured JSON body and dropped that half, so a
// production panic returned a generic 500 and vanished — no value, no stack,
// no file and line. The response is deliberately unchanged: the client gets
// "internal server error" and nothing about the internals, and the detail goes
// to the operator's log where it belongs.
//
// http.ErrAbortHandler is re-panicked rather than logged. The net/http server
// raises it to abort a response deliberately (a hijacked connection, a client
// that went away mid-write), and it is not an error — swallowing it here would
// convert a normal abort into a 500 the handler never asked for, and logging a
// stack for each one buries the panics that matter.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			if v == http.ErrAbortHandler {
				panic(v)
			}
			log.Printf("PANIC %s %s: %v\n%s", r.Method, r.URL.Path, v, debug.Stack())
			httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		}()
		next.ServeHTTP(w, r)
	})
}
