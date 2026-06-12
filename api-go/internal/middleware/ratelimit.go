package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
)

// Rate-limit budgets mirror .NET's RateLimitOptions defaults: a fixed 1-minute
// window, 60 requests for free users, 600 for paid. The in-memory store is
// correct because the Go app, like the .NET one, runs with MaxReplicas = 1
// (GH#418 parity behaviour 3); a distributed store would be needed only if it
// scaled out.
const (
	rateLimitWindow = time.Minute
	freeTierLimit   = 60
	paidTierLimit   = 600
)

// tierLookup reports whether a user is on a paid tier. The entitlement source is
// the Cosmos profile, NOT the JWT subscription_tier claim (ADR 0010): Cosmos is
// the single source of truth for entitlements. *profiles.CosmosStore-backed
// adapters satisfy this; tests substitute a fake.
type tierLookup interface {
	IsPaidUser(ctx context.Context, userID string) (bool, error)
}

// RateLimit returns middleware that enforces the per-user fixed-window limit.
// Anonymous requests (no subject in context) pass through unmetered, matching
// .NET's skip-when-no-sub behaviour. On an allowed request it sets
// X-RateLimit-Limit and X-RateLimit-Remaining; on a throttled one it returns 429
// with those headers (Remaining 0) plus Retry-After.
func RateLimit(store *rateLimitStore, tiers tierLookup, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := auth.Subject(r.Context())
			if userID == "" {
				next.ServeHTTP(w, r)
				return
			}

			limit := freeTierLimit
			paid, err := tiers.IsPaidUser(r.Context(), userID)
			if err != nil {
				// Fail open to the free limit: a transient profile-lookup failure
				// must never 500 an otherwise-valid request. The request still flows;
				// only the (lower) free budget applies.
				logger.WarnContext(r.Context(), "rate-limit tier lookup failed; using free limit",
					"userId", userID, "error", err)
			} else if paid {
				limit = paidTierLimit
			}

			result := store.checkAndIncrement(userID, limit)
			if !result.allowed {
				w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("Retry-After", strconv.Itoa(result.retryAfterSeconds()))
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(result.remaining))
			next.ServeHTTP(w, r)
		})
	}
}

// rateLimitResult is the outcome of one checkAndIncrement.
type rateLimitResult struct {
	allowed    bool
	remaining  int
	retryAfter time.Duration
}

// retryAfterSeconds rounds the retry-after up to whole seconds (minimum 1),
// matching .NET's Math.Ceiling on the TotalSeconds.
func (r rateLimitResult) retryAfterSeconds() int {
	secs := int(r.retryAfter / time.Second)
	if r.retryAfter%time.Second != 0 {
		secs++
	}
	if secs < 1 {
		secs = 1
	}
	return secs
}

// rateLimitStore is an in-memory sliding-window counter keyed by user id. It
// reproduces .NET's InMemoryRateLimitStore: a per-user queue of request
// timestamps, evicting entries older than the window on each check.
type rateLimitStore struct {
	now func() time.Time

	mu       sync.Mutex
	requests map[string][]time.Time
}

// newRateLimitStore builds a store using the given clock (injected for tests).
func newRateLimitStore(now func() time.Time) *rateLimitStore {
	return &rateLimitStore{now: now, requests: map[string][]time.Time{}}
}

// NewRateLimitStore builds the production store on the real clock. It returns
// the unexported concrete type so callers can only pass it back to RateLimit.
func NewRateLimitStore() *rateLimitStore {
	return newRateLimitStore(time.Now)
}

// checkAndIncrement evicts expired timestamps, then either records the request
// (returning the remaining budget) or denies it (returning the retry-after until
// the oldest in-window timestamp ages out).
func (s *rateLimitStore) checkAndIncrement(userID string, limit int) rateLimitResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	windowStart := now.Add(-rateLimitWindow)

	times := s.requests[userID]
	kept := times[:0]
	for _, t := range times {
		if t.After(windowStart) {
			kept = append(kept, t)
		}
	}

	if len(kept) >= limit {
		retryAfter := kept[0].Add(rateLimitWindow).Sub(now)
		if retryAfter < time.Millisecond {
			retryAfter = time.Millisecond
		}
		s.requests[userID] = kept
		return rateLimitResult{allowed: false, remaining: 0, retryAfter: retryAfter}
	}

	kept = append(kept, now)
	s.requests[userID] = kept
	return rateLimitResult{allowed: true, remaining: limit - len(kept)}
}
