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

// The limiter is the only piece of shared mutable request-path state in the
// service, so it is the one place a data race can live. Run with -race.
//
// Deliberately adversarial rather than merely concurrent. An earlier version
// of this test used 50 goroutines against a limit of 100 with a one-minute
// window, which never crossed the refusal path and never rolled the window —
// so it exercised the uncontended happy path and proved close to nothing.
//
// The window roll is the case worth proving: it REPLACES the map
// (rl.counts = make(...)) while other goroutines are inside allow(). A short
// window and a low limit make that happen many times under load.
func TestRateLimitIsSafeUnderConcurrency(t *testing.T) {
	const (
		goroutines = 64
		perG       = 40
	)
	// Low enough that most calls are refused, short enough that the window
	// rolls repeatedly while every goroutine is still running.
	rl := middleware.NewRateLimit(5, 2*time.Millisecond)
	h := rl.Handler(okHandler())

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		allowed int
		refused int
	)
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < perG; i++ {
				// A mix of shared and distinct keys, so goroutines contend on
				// the same bucket as well as inserting new ones into a map
				// that is being swapped underneath them.
				addr := "10.0.1." + strconv.Itoa(g%4) + ":5000"
				if i%3 == 0 {
					addr = "10.0.2." + strconv.Itoa(g) + ":5000"
				}
				code := request(h, addr).Code
				mu.Lock()
				if code == http.StatusOK {
					allowed++
				} else {
					refused++
				}
				mu.Unlock()
			}
		}(g)
	}
	wg.Wait()

	// Both paths have to have been taken, or the run proved only that the
	// uncontended case is safe.
	if allowed == 0 {
		t.Error("no request was allowed; the limiter never took the accept path")
	}
	if refused == 0 {
		t.Error("no request was refused; the limiter never took the reject path, " +
			"so the contended counter was never actually contended")
	}
	if got := allowed + refused; got != goroutines*perG {
		t.Errorf("counted %d outcomes, want %d — a request produced neither", got, goroutines*perG)
	}
}
