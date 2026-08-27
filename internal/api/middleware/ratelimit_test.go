package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shurikai/role-model/internal/api/middleware"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func request(h http.Handler, remoteAddr string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req.RemoteAddr = remoteAddr
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRateLimitAllowsUpToTheLimitThenRefuses(t *testing.T) {
	h := middleware.NewRateLimit(3, time.Minute).Handler(okHandler())

	for i := 1; i <= 3; i++ {
		if got := request(h, "10.0.0.1:5000").Code; got != http.StatusOK {
			t.Fatalf("request %d: got %d, want 200 — the limit is the number allowed, not the number before refusal", i, got)
		}
	}

	rec := request(h, "10.0.0.1:5000")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("request 4: got %d, want 429", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "rate_limited") {
		t.Errorf("body should carry the structured code, got: %s", rec.Body.String())
	}

	// A client that spins is the load this exists to shed, so it has to be
	// told when to come back.
	retry := rec.Header().Get("Retry-After")
	if retry == "" {
		t.Fatal("Retry-After must be set on a 429")
	}
	if n, err := strconv.Atoi(retry); err != nil || n <= 0 {
		t.Errorf("Retry-After = %q, want a positive number of seconds", retry)
	}
}

// Buckets are per client. One noisy address must not lock everyone else out —
// which would turn the limiter into the denial of service.
func TestRateLimitIsPerClient(t *testing.T) {
	h := middleware.NewRateLimit(2, time.Minute).Handler(okHandler())

	request(h, "10.0.0.1:5000")
	request(h, "10.0.0.1:5000")
	if got := request(h, "10.0.0.1:5000").Code; got != http.StatusTooManyRequests {
		t.Fatalf("the noisy client should be limited, got %d", got)
	}

	if got := request(h, "10.0.0.2:5000").Code; got != http.StatusOK {
		t.Errorf("a different client got %d, want 200", got)
	}
}

// The port varies per connection; the bucket must not.
func TestRateLimitIgnoresTheSourcePort(t *testing.T) {
	h := middleware.NewRateLimit(2, time.Minute).Handler(okHandler())

	request(h, "10.0.0.1:5000")
	request(h, "10.0.0.1:6001")
	if got := request(h, "10.0.0.1:7002").Code; got != http.StatusTooManyRequests {
		t.Fatalf("a new source port opened a fresh bucket, got %d", got)
	}
}

// X-Forwarded-For is set by the client. Trusting it would let anyone reset
// their own bucket by varying one header, which is worse than no limiter
// because it looks like protection.
func TestRateLimitDoesNotTrustForwardedHeaders(t *testing.T) {
	h := middleware.NewRateLimit(2, time.Minute).Handler(okHandler())

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		req.RemoteAddr = "10.0.0.1:5000"
		req.Header.Set("X-Forwarded-For", "1.2.3."+strconv.Itoa(i))
		h.ServeHTTP(httptest.NewRecorder(), req)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req.RemoteAddr = "10.0.0.1:5000"
	req.Header.Set("X-Forwarded-For", "9.9.9.9")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("a spoofed X-Forwarded-For opened a fresh bucket, got %d", rec.Code)
	}
}

func TestRateLimitResetsAfterTheWindow(t *testing.T) {
	h := middleware.NewRateLimit(1, 40*time.Millisecond).Handler(okHandler())

	if got := request(h, "10.0.0.1:5000").Code; got != http.StatusOK {
		t.Fatalf("first request got %d", got)
	}
	if got := request(h, "10.0.0.1:5000").Code; got != http.StatusTooManyRequests {
		t.Fatalf("second request got %d, want 429", got)
	}

	time.Sleep(60 * time.Millisecond)

	if got := request(h, "10.0.0.1:5000").Code; got != http.StatusOK {
		t.Errorf("after the window rolled over, got %d, want 200", got)
	}
}

// A per-client map that only grows is itself a denial-of-service vector — the
// class of bug this middleware exists to prevent. The window roll has to be
// the eviction too.
func TestRateLimitDoesNotAccumulateClientsAcrossWindows(t *testing.T) {
	rl := middleware.NewRateLimit(1, 30*time.Millisecond)
	h := rl.Handler(okHandler())

	for i := 0; i < 500; i++ {
		request(h, "10.0."+strconv.Itoa(i/256)+"."+strconv.Itoa(i%256)+":5000")
	}
	time.Sleep(50 * time.Millisecond)

	// One request after the window rolls sweeps the previous window's map. If
	// entries survived, this client would be sharing a map with 500 others —
	// observable only through the reset, which is what is asserted here.
	if got := request(h, "10.9.9.9:5000").Code; got != http.StatusOK {
		t.Fatalf("got %d, want 200", got)
	}
	if got := request(h, "10.9.9.9:5000").Code; got != http.StatusTooManyRequests {
		t.Fatalf("the new window is not counting: got %d, want 429", got)
	}
}

// The counter is shared across every request the server handles, so it is
// reached concurrently by construction. Run with -race.
func TestRateLimitIsSafeUnderConcurrency(t *testing.T) {
	h := middleware.NewRateLimit(100, time.Minute).Handler(okHandler())

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			request(h, "10.0.1."+strconv.Itoa(i%10)+":5000")
		}(i)
	}
	wg.Wait()
}
