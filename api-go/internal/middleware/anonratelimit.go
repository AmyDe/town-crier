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

// AnonRateLimit returns middleware enforcing a fixed-window rate limit keyed
// by resolved client IP (internal/clientip), applied ONLY to requests with no
// authenticated subject (auth.Subject(ctx) == ""). Authenticated traffic
// passes straight through untouched: it is metered by the sibling per-subject
// RateLimit instead, and this middleware never inspects or affects it
// (GH#868 Phase 1). A point+radius public endpoint is a whole-table scraping
// target and drives unmetered load onto PlanIt (our free, single-operator
// upstream) if left uncapped, hence this second limiter.
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
// The client IP itself is never logged, stored, or exported anywhere beyond
// this in-memory accounting: only the numeric limit/retry values are recorded
// on a throttle, honouring the clientip package's "in-memory, non-persisted
// purpose only" constraint.
func AnonRateLimit(store *anonRateLimitStore, limit int, logger *slog.Logger) func(http.Handler) http.Handler {
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

			addr := clientip.FromRequest(r)
			result := store.checkAndIncrement(addr, limit)
			if !result.allowed {
				w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("Retry-After", strconv.Itoa(result.retryAfterSeconds()))
				logger.WarnContext(r.Context(), "anonymous rate limit exceeded",
					"limit", limit, "retryAfterSeconds", result.retryAfterSeconds())
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(result.remaining))
			next.ServeHTTP(w, r)
		})
	}
}

// anonRateLimitStore is an in-memory sliding-window counter keyed by resolved
// client IP: a per-IP queue of request timestamps, evicting entries older than
// the window on every check. It mirrors rateLimitStore's mechanism (see
// ratelimit.go) generalised to a configurable window and a netip.Addr key
// instead of a user-id string — netip.Addr is a small comparable value, so it
// works directly as a map key with no string conversion (and therefore no
// temptation to log it).
//
// Eviction is mandatory from day one (regression guard for GH#518, where a
// per-subject map once grew unbounded because keys were never reclaimed): once
// an IP's timestamps have all aged out of the window, the map key is deleted
// rather than left holding an empty slice. This matters even more here than
// for the per-subject store: the anonymous caller population is unbounded
// (any IP on the internet), not the bounded set of registered users.
type anonRateLimitStore struct {
	now    func() time.Time
	window time.Duration

	mu       sync.Mutex
	requests map[netip.Addr][]time.Time
}

// newAnonRateLimitStore builds a store using the given clock and window
// (injected for tests).
func newAnonRateLimitStore(now func() time.Time, window time.Duration) *anonRateLimitStore {
	return &anonRateLimitStore{now: now, window: window, requests: map[netip.Addr][]time.Time{}}
}

// NewAnonRateLimitStore builds the production store on the real clock with the
// given fixed window. It returns the unexported concrete type so callers can
// only pass it back to AnonRateLimit.
func NewAnonRateLimitStore(window time.Duration) *anonRateLimitStore {
	return newAnonRateLimitStore(time.Now, window)
}

// checkAndIncrement evicts expired timestamps for addr, then either records
// the request (returning the remaining budget) or denies it (returning the
// retry-after until the oldest in-window timestamp ages out). See
// rateLimitStore.checkAndIncrement for the identical compaction/eviction
// reasoning this mirrors.
func (s *anonRateLimitStore) checkAndIncrement(addr netip.Addr, limit int) rateLimitResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	windowStart := now.Add(-s.window)

	times := s.requests[addr]
	kept := times[:0]
	for _, t := range times {
		if t.After(windowStart) {
			kept = append(kept, t)
		}
	}

	if len(kept) == 0 {
		delete(s.requests, addr)
	} else {
		s.requests[addr] = kept
	}

	if len(kept) >= limit {
		var retryAfter time.Duration
		if len(kept) > 0 {
			retryAfter = kept[0].Add(s.window).Sub(now)
		}
		if retryAfter < time.Millisecond {
			retryAfter = time.Millisecond
		}
		return rateLimitResult{allowed: false, remaining: 0, retryAfter: retryAfter}
	}

	kept = append(kept, now)
	s.requests[addr] = kept
	return rateLimitResult{allowed: true, remaining: limit - len(kept)}
}
