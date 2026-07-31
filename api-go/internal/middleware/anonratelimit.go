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
// user data, are not part of the scraping surface this middleware defends
// (GH#868 Phase 1), and metering them would only spend budget a genuine
// anonymous caller could otherwise use. This is the chosen convention for the
// ACA-probe exemption; keep it in lockstep with internal/health's registered
// paths.
var healthCheckPaths = map[string]struct{}{
	"/health":    {},
	"/v1/health": {},
}

// AnonRateLimit returns middleware enforcing a per-IP token-bucket rate limit
// keyed by resolved client IP (internal/clientip), applied ONLY to requests
// with no authenticated subject (auth.Subject(ctx) == ""). Authenticated
// traffic passes straight through untouched: it is metered by the sibling
// per-subject RateLimit instead, and this middleware never inspects or
// affects it (GH#868 Phase 1). A point+radius public endpoint is a
// whole-table scraping target and drives unmetered load onto PlanIt (our
// free, single-operator upstream) if left uncapped, hence this second
// limiter.
//
// burst is the bucket's capacity: the number of requests an IP can make in
// an instantaneous burst before it starts drawing on the steady refill rate
// carried by store (see NewAnonRateLimitStore). This replaced a flat
// fixed-window counter (tc-vwobf): a real anonymous session panning the map
// fans a single pan/zoom settle event out across several endpoints
// (GET /v1/applications/clusters, GET /v1/me/watch-zones/{id}/applications/
// clusters, GET /v1/applications/near-point, ...) even though both clients
// already debounce pan/zoom client-side at ~250ms — so a flat per-window
// budget throttled ordinary browsing, not just scraping. A token bucket lets
// that fan-out through as a burst while the refill rate still caps
// sustained/scraping-shaped load at roughly the same long-run rate as before
// (see the config doc comment for the chosen numbers).
//
// Requests whose client IP cannot be resolved (clientip.FromRequest returns
// the invalid zero Addr — which per the clientip package doc happens only
// when RemoteAddr itself cannot be parsed) collapse onto one shared
// conservative bucket: the zero Addr is identical for every such request, so
// no caller that cannot be individually attributed can bypass metering
// entirely by virtue of being unattributable.
//
// On an allowed request it sets X-RateLimit-Limit and X-RateLimit-Remaining;
// on a throttled one it returns 429 with those headers (Remaining 0) plus
// Retry-After — the same response contract as the per-subject RateLimit.
//
// exempt is an additional per-request bypass alongside the health-check paths
// above, evaluated only for otherwise-anonymous requests: when it returns
// true the request passes straight through untouched, exactly like
// authenticated traffic. nil means no additional exemption. The motivating
// case (GH#872 collateral, tc-zod82) is the build-time SEO endpoints
// (GET /v1/authorities/{id}/applications, GET /v1/applications/near): they
// authenticate via the X-Build-Key header checked inside the handler
// (applications.requireBuildKey), not via Auth0, so auth.Subject is always
// empty for them and the CI seo-refresh job's ~418 sequential requests from
// one runner IP were being metered as anonymous and 429ing. Callers build the
// predicate from applications.BuildKeyMatches (cmd/api/wiring.go) rather than
// this package importing internal/applications directly, keeping middleware
// dependency-free.
//
// The client IP itself is never logged, stored, or exported anywhere beyond
// this in-memory accounting: only the numeric limit/retry values are recorded
// on a throttle, honouring the clientip package's "in-memory, non-persisted
// purpose only" constraint.
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
// Eviction is mandatory from day one (regression guard for GH#518, where a
// per-subject map once grew unbounded because keys were never reclaimed): the
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
	// in the map forever (GH#518 regression guard, token-bucket analogue of
	// the sliding-window's "all timestamps aged out" eviction).
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
