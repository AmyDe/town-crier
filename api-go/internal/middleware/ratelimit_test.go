package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
)

// slogDiscard returns a logger that drops all output, for tests that only assert
// on responses, not logs.
func slogDiscard() *slog.Logger { return slog.New(slog.DiscardHandler) }

// fakeTierLookup returns a fixed paid/free decision per user, standing in for
// the Cosmos profile lookup the real middleware uses.
type fakeTierLookup struct {
	paid map[string]bool
	err  error
}

func (f *fakeTierLookup) IsPaidUser(_ context.Context, userID string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.paid[userID], nil
}

// fixedClock returns a controllable time for window math.
type fixedClock struct{ t time.Time }

func (c *fixedClock) now() time.Time { return c.t }

func authedRequest(sub string) *http.Request {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/me", nil)
	return r.WithContext(auth.WithSubject(r.Context(), sub))
}

func TestRateLimit_AllowedRequestSetsHeaders(t *testing.T) {
	t.Parallel()

	clock := &fixedClock{t: time.Unix(1000, 0)}
	store := newRateLimitStore(clock.now)
	mw := RateLimit(store, &fakeTierLookup{}, slogDiscard())

	rec := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rec, authedRequest("auth0|abc"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("X-RateLimit-Limit"); got != "60" {
		t.Errorf("X-RateLimit-Limit: got %q, want 60 (free)", got)
	}
	if got := rec.Header().Get("X-RateLimit-Remaining"); got != "59" {
		t.Errorf("X-RateLimit-Remaining: got %q, want 59", got)
	}
}

func TestRateLimit_PaidTierGetsHigherLimit(t *testing.T) {
	t.Parallel()

	clock := &fixedClock{t: time.Unix(1000, 0)}
	store := newRateLimitStore(clock.now)
	tiers := &fakeTierLookup{paid: map[string]bool{"auth0|pro": true}}
	mw := RateLimit(store, tiers, slogDiscard())

	rec := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rec, authedRequest("auth0|pro"))

	if got := rec.Header().Get("X-RateLimit-Limit"); got != "600" {
		t.Errorf("X-RateLimit-Limit: got %q, want 600 (paid)", got)
	}
	if got := rec.Header().Get("X-RateLimit-Remaining"); got != "599" {
		t.Errorf("X-RateLimit-Remaining: got %q, want 599", got)
	}
}

func TestRateLimit_AnonymousRequestSkipped(t *testing.T) {
	t.Parallel()

	clock := &fixedClock{t: time.Unix(1000, 0)}
	store := newRateLimitStore(clock.now)
	mw := RateLimit(store, &fakeTierLookup{}, slogDiscard())

	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/health", nil)
	rec := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if rec.Header().Get("X-RateLimit-Limit") != "" {
		t.Errorf("anonymous request should carry no rate-limit headers, got %q", rec.Header().Get("X-RateLimit-Limit"))
	}
}

func TestRateLimit_ExceededReturns429WithHeaders(t *testing.T) {
	t.Parallel()

	clock := &fixedClock{t: time.Unix(1000, 0)}
	store := newRateLimitStore(clock.now)
	mw := RateLimit(store, &fakeTierLookup{}, slogDiscard())
	h := mw(okHandler())

	// Exhaust the 60-request free budget.
	for i := range 60 {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, authedRequest("auth0|abc"))
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: got %d, want 200", i, rec.Code)
		}
	}

	// 61st request in the same window is throttled.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, authedRequest("auth0|abc"))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("61st request: got %d, want 429", rec.Code)
	}
	if got := rec.Header().Get("X-RateLimit-Limit"); got != "60" {
		t.Errorf("429 X-RateLimit-Limit: got %q, want 60", got)
	}
	if got := rec.Header().Get("X-RateLimit-Remaining"); got != "0" {
		t.Errorf("429 X-RateLimit-Remaining: got %q, want 0", got)
	}
	if got := rec.Header().Get("Retry-After"); got == "" || got == "0" {
		t.Errorf("429 Retry-After: got %q, want a positive seconds value", got)
	}
}

func TestRateLimit_WindowResets(t *testing.T) {
	t.Parallel()

	clock := &fixedClock{t: time.Unix(1000, 0)}
	store := newRateLimitStore(clock.now)
	mw := RateLimit(store, &fakeTierLookup{}, slogDiscard())
	h := mw(okHandler())

	for range 60 {
		h.ServeHTTP(httptest.NewRecorder(), authedRequest("auth0|abc"))
	}
	// Throttled now.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, authedRequest("auth0|abc"))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected throttle before window reset, got %d", rec.Code)
	}

	// Advance past the 1-minute window; the budget refreshes.
	clock.t = clock.t.Add(61 * time.Second)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, authedRequest("auth0|abc"))
	if rec2.Code != http.StatusOK {
		t.Errorf("after window reset: got %d, want 200", rec2.Code)
	}
}

func TestRateLimit_TierLookupFailureFailsOpenToFree(t *testing.T) {
	t.Parallel()

	clock := &fixedClock{t: time.Unix(1000, 0)}
	store := newRateLimitStore(clock.now)
	tiers := &fakeTierLookup{err: context.DeadlineExceeded}
	mw := RateLimit(store, tiers, slogDiscard())

	rec := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rec, authedRequest("auth0|abc"))

	// A profile lookup failure must not 500 the request; it falls back to the
	// free limit (the request still flows).
	if rec.Code != http.StatusOK {
		t.Fatalf("status on tier-lookup failure: got %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("X-RateLimit-Limit"); got != "60" {
		t.Errorf("fallback limit: got %q, want 60 (free)", got)
	}
}

// hasKey is a test-only accessor that reports whether the store map currently
// holds an entry for userID. It must not be used in production paths.
func (s *rateLimitStore) hasKey(userID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.requests[userID]
	return ok
}

// TestRateLimitStore_EvictsIdleUserKey verifies that once all of a user's
// in-window timestamps have aged out and the next checkAndIncrement call
// evicts them, the store reclaims the map key (delete) rather than writing
// back an empty slice.
//
// The denied path (limit=0) is used here because it is the only code path
// that writes back without appending a new timestamp, making key deletion
// observable in a single call. Active-user behaviour (limit>0, allowed path)
// is covered by the existing window/limit tests above.
func TestRateLimitStore_EvictsIdleUserKey(t *testing.T) {
	t.Parallel()

	const userID = "auth0|idle"

	clock := &fixedClock{t: time.Unix(1000, 0)}
	store := newRateLimitStore(clock.now)

	// Seed one timestamp at t=0 so the user has a map entry.
	store.checkAndIncrement(userID, freeTierLimit)
	if !store.hasKey(userID) {
		t.Fatal("setup: expected key to be present after first request")
	}

	// Advance past the window so the seeded timestamp is now expired.
	clock.t = clock.t.Add(rateLimitWindow + time.Second)

	// Use limit=0 to force the denied path: len(kept)==0 after eviction and
	// 0>=0 is true, so we land on the denied branch without appending a new
	// timestamp. The fix should delete the key instead of writing back an
	// empty slice.
	store.checkAndIncrement(userID, 0)

	if store.hasKey(userID) {
		t.Errorf("expected map key to be deleted after all timestamps aged out, but key is still present")
	}
}
