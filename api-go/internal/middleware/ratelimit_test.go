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

// keyLen is a test-only accessor that reports the number of timestamps stored
// for userID (0 if the key is absent). It must not be used in production paths.
func (s *rateLimitStore) keyLen(userID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.requests[userID])
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

// TestRateLimitStore_DeniedDoesNotDoubleCountAcrossCalls guards against an
// in-place compaction regression. checkAndIncrement filters surviving
// timestamps into kept := times[:0], which aliases the stored slice's backing
// array; the map header is only refreshed when kept is reassigned to
// s.requests[userID]. On a real over-limit denial where the filter actually
// drops an expired head entry, the compacted (shorter) slice MUST be persisted.
// Otherwise the stored header keeps its old length with a stale duplicated tail,
// and the next call re-reads — and re-counts — those tail entries, denying the
// user for longer than the true window.
func TestRateLimitStore_DeniedDoesNotDoubleCountAcrossCalls(t *testing.T) {
	t.Parallel()

	const userID = "auth0|busy"

	clock := &fixedClock{t: time.Unix(1000, 0)}
	store := newRateLimitStore(clock.now)

	// Seed three timestamps with a generous limit so each is recorded. They are
	// placed so that, after the window advance below, exactly the first one ages
	// out and the latter two survive: stored slice = [t1000, t1030, t1031],
	// len 3.
	for _, sec := range []int64{1000, 1030, 1031} {
		clock.t = time.Unix(sec, 0)
		if got := store.checkAndIncrement(userID, 100); !got.allowed {
			t.Fatalf("seed request at %d: expected allowed", sec)
		}
	}
	if got := store.keyLen(userID); got != 3 {
		t.Fatalf("after seeding: stored len = %d, want 3", got)
	}

	// Advance to t=1061. windowStart = 1061 - 60s = 1001, so only t1000 ages
	// out; t1030 and t1031 stay in-window. Filtering will drop one entry ->
	// kept has len 2.
	clock.t = time.Unix(1061, 0)

	// Over-limit denial (limit=1, two in-window entries survive). The filter
	// drops the expired head, so the persisted slice must shrink to len 2. With
	// the aliasing bug the header would stay len 3 with a duplicated tail.
	if got := store.checkAndIncrement(userID, 1); got.allowed {
		t.Fatal("first over-limit request: expected denial")
	}
	if got := store.keyLen(userID); got != 2 {
		t.Fatalf("after first denial: stored len = %d, want 2 (expired head dropped; compacted slice must be persisted)", got)
	}

	// A second denial in the same window must not grow or re-count: still
	// exactly the two genuine in-window timestamps.
	if got := store.checkAndIncrement(userID, 1); got.allowed {
		t.Fatal("second over-limit request: expected denial")
	}
	if got := store.keyLen(userID); got != 2 {
		t.Fatalf("after second denial: stored len = %d, want 2 (double-count regression)", got)
	}
}
