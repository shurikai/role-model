package middleware_test

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shurikai/role-model/internal/api/middleware"
)

// captureLog redirects the standard logger for one test and returns what was
// written to it.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	flags, out := log.Flags(), log.Writer()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(out); log.SetFlags(flags) })
	return &buf
}

// The whole point of the change: a panic in production used to return a
// generic 500 and vanish, with no value, no stack, and no file and line.
func TestRecovererLogsThePanicAndItsStack(t *testing.T) {
	buf := captureLog(t)

	h := middleware.Recoverer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("contribution assembler exploded")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/applications/x/generate", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}

	logged := buf.String()
	if !strings.Contains(logged, "contribution assembler exploded") {
		t.Errorf("the panic value must be logged, got: %q", logged)
	}
	// A stack with no frame from this file is not a stack worth having.
	if !strings.Contains(logged, "recover_test.go") {
		t.Errorf("the stack must reach the panicking frame, got: %q", logged)
	}
	// Which request panicked is most of the value when one endpoint is bad.
	if !strings.Contains(logged, "/api/v1/applications/x/generate") {
		t.Errorf("the request path must be logged, got: %q", logged)
	}
}

// The body is unchanged on purpose. The client learns nothing about the
// internals; the detail goes to the operator's log.
func TestRecovererDoesNotLeakThePanicToTheClient(t *testing.T) {
	captureLog(t)

	h := middleware.Recoverer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("secret-bearing value: postgres://user:hunter2@db/prod")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/employers", nil))

	if body := rec.Body.String(); strings.Contains(body, "hunter2") {
		t.Fatalf("the panic value must not reach the client, got: %s", body)
	}
	if !strings.Contains(rec.Body.String(), "internal_error") {
		t.Errorf("body should carry the structured error code, got: %s", rec.Body.String())
	}
}

// net/http raises ErrAbortHandler to abort a response deliberately. Catching it
// would turn a normal abort into a 500 the handler never asked for, and log a
// stack for each one.
func TestRecovererRepanicsOnErrAbortHandler(t *testing.T) {
	captureLog(t)

	h := middleware.Recoverer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	}))

	defer func() {
		if v := recover(); v != http.ErrAbortHandler {
			t.Fatalf("ErrAbortHandler must propagate, recovered: %v", v)
		}
	}()
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/health", nil))
	t.Fatal("expected the panic to propagate")
}

func TestRecovererPassesNonPanickingRequestsThrough(t *testing.T) {
	h := middleware.Recoverer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusTeapot {
		t.Errorf("status = %d, want the handler's own 418", rec.Code)
	}
}
