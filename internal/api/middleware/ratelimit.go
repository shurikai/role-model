package middleware

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/shurikai/role-model/internal/httputil"
)

// RateLimit is a fixed-window per-client limiter.
//
// # Why this is hand-written
//
// The obvious answer is go-chi/httprate, and it is a good library. It also
// pulls in three modules — xxh3, cpuid and x/sys — for hashing throughput this
// service will never approach, and the Go side of this project keeps a
// deliberately conservative dependency rule. What is actually needed here is a
// counter, a map and a mutex.
//
// # What it protects
//
// Two different things, at two different rates:
//
//   - The auth routes. /auth/login runs bcrypt at DefaultCost on every
//     attempt, which is expensive by design and therefore a denial-of-service
//     lever as well as a password-guessing one. /auth/signup creates rows.
//   - The five endpoints that spend the operator's Anthropic key. Those are
//     behind a valid JWT, but signup is open by default in development, and a
//     single scripted loop against POST /import/career drains a key.
//
// # Fixed window, not sliding
//
// A fixed window lets a caller send 2N requests across a window boundary. That
// is a real property and it is acceptable here: the point is to make a
// scripted loop useless, not to enforce a precise rate. A sliding window costs
// a ring buffer per client to fix a factor of two.
//
// # Bounding the map
//
// A per-client map that only ever grows is itself a denial-of-service vector —
// the exact class of bug this middleware exists to prevent. Entries are swept
// on write whenever the window rolls over, so memory is proportional to
// clients seen in one window rather than to clients ever seen.
type RateLimit struct {
	limit  int
	window time.Duration

	mu      sync.Mutex
	counts  map[string]int
	resetAt time.Time
}

// NewRateLimit returns middleware allowing limit requests per client per
// window.
func NewRateLimit(limit int, window time.Duration) *RateLimit {
	return &RateLimit{
		limit:   limit,
		window:  window,
		counts:  make(map[string]int),
		resetAt: time.Now().Add(window),
	}
}

// Handler wraps next, rejecting a client that has exceeded the limit.
func (rl *RateLimit) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		retryAfter, ok := rl.allow(clientKey(r))
		if !ok {
			// Retry-After is the difference between a client that backs off
			// and one that spins, and a spinning client is the load this is
			// trying to shed.
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())+1))
			httputil.WriteError(w, http.StatusTooManyRequests, "rate_limited",
				"too many requests; try again shortly")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// allow records a request and reports whether it is permitted, along with how
// long remains in the current window.
func (rl *RateLimit) allow(key string) (time.Duration, bool) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	if now.After(rl.resetAt) {
		// The sweep. Dropping the whole map is both the window reset and the
		// eviction, so nothing accumulates across windows.
		rl.counts = make(map[string]int)
		rl.resetAt = now.Add(rl.window)
	}

	rl.counts[key]++
	if rl.counts[key] > rl.limit {
		return rl.resetAt.Sub(now), false
	}
	return 0, true
}

// clientKey identifies the caller.
//
// Deliberately the socket's remote address and NOT X-Forwarded-For. A header
// is set by the client, so trusting it lets anyone reset their own bucket by
// varying one string — which is worse than no limiter, because it looks like
// protection. A deployment behind a proxy needs the proxy's own limiter, or a
// trusted-proxy configuration that does not exist here yet.
func clientKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
