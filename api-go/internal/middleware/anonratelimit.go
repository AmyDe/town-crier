package middleware

import (
	"log/slog"
	"net/http"
	"net/netip"
	"strconv"
	"sync"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
	"github.com/AmyDe/town-crier/api-go/internal/clientip"
)

// healthCheckPaths are the liveness/readiness probe paths Container Apps hits
// continuously. They are exempt from anonymous rate limiting: they carry no
// user data, are not part of the scraping surface this middleware defends,
// and metering them would only spend budget a genuine
// anonymous caller could otherwise use. This is the chosen convention for the
// ACA-probe exemption; keep it in lockstep with internal/health's registered
// paths.
var healthCheckPaths = map[string]struct{}{
	"/health":    {},
	"/v1/health": {},
}

// AnonRateLimit returns middleware enforcing a per-IP token-bucket rate limit
// keyed by resolved client IP (internal/clientip), applied only to requests
// with no authenticated subject. Authenticated traffic passes straight through
// untouched (the per-subject RateLimit meters it).
//
// burst is the bucket's capacity: the number of requests an IP can make in an
// instantaneous burst before drawing on the steady refill rate carried by
// store. A token bucket, not a flat window, because one anonymous map pan/zoom
// fans out across several endpoints at once even with client-side debounce.
//
// Requests whose client IP cannot be resolved share one bucket (the zero
// Addr), so being unattributable never bypasses metering.
//
// On an allowed request it sets X-RateLimit-Limit and X-RateLimit-Remaining;
// on a throttled one it returns 429 with those headers (Remaining 0) plus
// Retry-After, the same response contract as RateLimit.
//
// exempt is an additional per-request bypass alongside the health-check paths,
// evaluated only for otherwise-anonymous requests; nil means none. It exists
// for the build-key-authenticated SEO endpoints, whose auth.Subject is always
// empty.
//
// The client IP is never logged, stored, or exported beyond this in-memory
// accounting.
func AnonRateLimit(store *anonRateLimitStore, burst int, exempt func(*http.Request) bool, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if auth.Subject(r.Context()) != "" {
				next.ServeHTTP(w, r)
				return
			}
			if _, ok := healthCheckPaths[r.URL.Path]; ok {
				next.ServeHTTP(w, r)
				return
			}
			if exempt != nil && exempt(r) {
				next.ServeHTTP(w, r)
				return
			}

			addr := clientip.FromRequest(r)
			result := store.checkAndIncrement(addr, burst)
			if !result.allowed {
				w.Header().Set("X-RateLimit-Limit", strconv.Itoa(burst))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("Retry-After", strconv.Itoa(result.retryAfterSeconds()))
				logger.WarnContext(r.Context(), "anonymous rate limit exceeded",
					"limit", burst, "retryAfterSeconds", result.retryAfterSeconds())
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(burst))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(result.remaining))
			next.ServeHTTP(w, r)
		})
	}
}

// anonBucket is one IP's token-bucket state as of lastRefill. Tokens are
// computed lazily on each check — elapsed time since lastRefill multiplied by
// the store's refill rate — rather than on a background ticker, so an IP that
// never returns costs nothing beyond the map entry itself, and no goroutine
// needs a shutdown path.
type anonBucket struct {
	tokens     float64
	lastRefill time.Time
}

// anonRateLimitStore is an in-memory token bucket keyed by resolved client
// IP: each IP gets a bucket that starts full (at the per-call burst
// capacity), drains by one token per allowed request, and refills at a
// steady rate (refillPerSecond, injected at construction so tests can pin it)
// independent of any fixed window boundary. netip.Addr is a small comparable
// value, so it works directly as a map key with no string conversion (and
// therefore no temptation to log it).
//
// Eviction is mandatory, or the per-IP map grows unbounded: the
// token-bucket analogue of "all timestamps aged out of the window" is "the
// bucket has refilled all the way back to capacity" — that state carries no
// information beyond "no entry", so once a check computes a fully-refilled
// bucket the key is deleted rather than left sitting in memory holding a full
// bucket forever. This matters even more here than for the per-subject
// store: the anonymous caller population is unbounded (any IP on the
// internet), not the bounded set of registered users.
type anonRateLimitStore struct {
	now             func() time.Time
	refillPerSecond float64

	mu      sync.Mutex
	buckets map[netip.Addr]anonBucket
}

// newAnonRateLimitStore builds a store using the given clock and refill rate
// (tokens added per second), injected for tests.
func newAnonRateLimitStore(now func() time.Time, refillPerSecond float64) *anonRateLimitStore {
	return &anonRateLimitStore{now: now, refillPerSecond: refillPerSecond, buckets: map[netip.Addr]anonBucket{}}
}

// NewAnonRateLimitStore builds the production store on the real clock with
// the given steady refill rate (tokens/second).
func NewAnonRateLimitStore(refillPerSecond float64) *anonRateLimitStore {
	return newAnonRateLimitStore(time.Now, refillPerSecond)
}

// checkAndIncrement refills addr's bucket for elapsed time since its last
// touch (capped at burst, the per-call capacity), then either spends one
// token (returning the remaining balance) or denies the request (returning
// the retry-after until one token is earned back). See rateLimitStore.
// checkAndIncrement for the sibling per-subject store this generalises from a
// fixed window to continuous refill.
func (s *anonRateLimitStore) checkAndIncrement(addr netip.Addr, burst int) rateLimitResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	capacity := float64(burst)

	tokens := capacity
	if b, ok := s.buckets[addr]; ok {
		tokens = b.tokens
		if s.refillPerSecond > 0 {
			if elapsed := now.Sub(b.lastRefill).Seconds(); elapsed > 0 {
				tokens += elapsed * s.refillPerSecond
			}
		}
		if tokens > capacity {
			tokens = capacity
		}
	}

	// A bucket back at full capacity carries no information beyond "no
	// entry" — reclaim it now rather than leave a stale full bucket sitting
	// in the map forever.
	if tokens >= capacity {
		delete(s.buckets, addr)
	}

	if tokens < 1 {
		var retryAfter time.Duration
		if s.refillPerSecond > 0 {
			retryAfter = time.Duration((1 - tokens) / s.refillPerSecond * float64(time.Second))
		}
		if retryAfter < time.Millisecond {
			retryAfter = time.Millisecond
		}
		return rateLimitResult{allowed: false, remaining: 0, retryAfter: retryAfter}
	}

	tokens--
	s.buckets[addr] = anonBucket{tokens: tokens, lastRefill: now}
	return rateLimitResult{allowed: true, remaining: int(tokens)}
}
