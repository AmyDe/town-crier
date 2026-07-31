package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
)

// anonRequestWithPath builds a GET request carrying no Authorization header
// (so auth.Subject is empty), driving AnonRateLimit's default anonymous path.
// remoteAddr sets the TCP peer (r.RemoteAddr); an optional CF-Connecting-IP
// header can be layered on top via cfHeader.
func anonRequestWithPath(path, remoteAddr, cfHeader string) *http.Request {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
	r.RemoteAddr = remoteAddr
	if cfHeader != "" {
		r.Header.Set("CF-Connecting-IP", cfHeader)
	}
	return r
}

// anonRequest is anonRequestWithPath for an ordinary (non-health-check) route.
func anonRequest(remoteAddr string) *http.Request {
	return anonRequestWithPath("/v1/applications/search", remoteAddr, "")
}

func TestAnonRateLimit_AllowedRequestSetsHeaders(t *testing.T) {
	t.Parallel()

	clock := &fixedClock{t: time.Unix(2000, 0)}
	store := newAnonRateLimitStore(clock.now, 1)
	mw := AnonRateLimit(store, 60, nil, slogDiscard())

	rec := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rec, anonRequest("203.0.113.10:51000"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("X-RateLimit-Limit"); got != "60" {
		t.Errorf("X-RateLimit-Limit: got %q, want 60", got)
	}
	if got := rec.Header().Get("X-RateLimit-Remaining"); got != "59" {
		t.Errorf("X-RateLimit-Remaining: got %q, want 59", got)
	}
}

func TestAnonRateLimit_RequestsUpToBurstCapacitySucceed(t *testing.T) {
	t.Parallel()

	clock := &fixedClock{t: time.Unix(2000, 0)}
	store := newAnonRateLimitStore(clock.now, 1)
	mw := AnonRateLimit(store, 5, nil, slogDiscard())
	h := mw(okHandler())

	for i := range 5 {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, anonRequest("203.0.113.12:51000"))
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: got %d, want 200 (within burst capacity)", i, rec.Code)
		}
	}
}

func TestAnonRateLimit_ExceedsBurstReturns429WithHeaders(t *testing.T) {
	t.Parallel()

	clock := &fixedClock{t: time.Unix(2000, 0)}
	store := newAnonRateLimitStore(clock.now, 1)
	mw := AnonRateLimit(store, 60, nil, slogDiscard())
	h := mw(okHandler())

	for i := range 60 {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, anonRequest("203.0.113.11:51000"))
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: got %d, want 200", i, rec.Code)
		}
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, anonRequest("203.0.113.11:51000"))
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

// TestAnonRateLimit_AuthenticatedRequestPassesThrough is the acceptance-
// criterion test: authenticated traffic must never be touched by the
// anonymous limiter, even when it shares an IP with an anonymous caller who
// has exhausted the (deliberately tiny, burst=1) anonymous budget.
func TestAnonRateLimit_AuthenticatedRequestPassesThrough(t *testing.T) {
	t.Parallel()

	clock := &fixedClock{t: time.Unix(2000, 0)}
	store := newAnonRateLimitStore(clock.now, 1)
	mw := AnonRateLimit(store, 1, nil, slogDiscard())
	h := mw(okHandler())

	for i := range 5 {
		r := anonRequest("203.0.113.13:51000")
		r = r.WithContext(auth.WithSubject(r.Context(), "auth0|abc"))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		if rec.Code != http.StatusOK {
			t.Fatalf("authenticated request %d: got %d, want 200", i, rec.Code)
		}
		if got := rec.Header().Get("X-RateLimit-Limit"); got != "" {
			t.Errorf("authenticated request %d: got anon rate-limit header %q, want none", i, got)
		}
	}

	// The same IP, now making its first genuinely anonymous request, still has
	// its full budget — proving the authenticated loop above never touched the
	// IP's bucket.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, anonRequest("203.0.113.13:51000"))
	if rec.Code != http.StatusOK {
		t.Fatalf("first anonymous request on the IP: got %d, want 200", rec.Code)
	}
}

// TestAnonRateLimit_ExemptPredicatePassesThroughUnmetered is the acceptance
// test for the build-key exemption (GH#872 collateral, tc-zod82): a request
// the exempt predicate recognises must pass straight through even when the
// per-IP budget is exhausted, with no X-RateLimit headers set and no budget
// consumed — exactly like authenticated (Auth0-subject) traffic above.
func TestAnonRateLimit_ExemptPredicatePassesThroughUnmetered(t *testing.T) {
	t.Parallel()

	clock := &fixedClock{t: time.Unix(2000, 0)}
	store := newAnonRateLimitStore(clock.now, 1)
	exempt := func(r *http.Request) bool { return r.Header.Get("X-Build-Key") == "s3cret" }
	mw := AnonRateLimit(store, 1, exempt, slogDiscard())
	h := mw(okHandler())

	exemptRequest := func() *http.Request {
		r := anonRequest("203.0.113.70:51000")
		r.Header.Set("X-Build-Key", "s3cret")
		return r
	}

	for i := range 5 {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, exemptRequest())
		if rec.Code != http.StatusOK {
			t.Fatalf("exempt request %d: got %d, want 200", i, rec.Code)
		}
		if got := rec.Header().Get("X-RateLimit-Limit"); got != "" {
			t.Errorf("exempt request %d: got rate-limit header %q, want none", i, got)
		}
	}

	// The same IP, now making its first genuinely anonymous (keyless) request,
	// still has its full budget — proving the exempt loop above never touched
	// the IP's bucket.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, anonRequest("203.0.113.70:51000"))
	if rec.Code != http.StatusOK {
		t.Fatalf("first anonymous (keyless) request on the IP: got %d, want 200", rec.Code)
	}
}

// TestAnonRateLimit_ExemptPredicateFalseIsMeteredNormally confirms the
// exemption is opt-in per request: traffic the predicate does not recognise
// (wrong or absent build key) is metered exactly as it was before the
// predicate existed.
func TestAnonRateLimit_ExemptPredicateFalseIsMeteredNormally(t *testing.T) {
	t.Parallel()

	clock := &fixedClock{t: time.Unix(2000, 0)}
	store := newAnonRateLimitStore(clock.now, 1)
	exempt := func(r *http.Request) bool { return r.Header.Get("X-Build-Key") == "s3cret" }
	mw := AnonRateLimit(store, 1, exempt, slogDiscard())
	h := mw(okHandler())

	first := httptest.NewRecorder()
	h.ServeHTTP(first, anonRequest("203.0.113.71:51000"))
	if first.Code != http.StatusOK {
		t.Fatalf("first (keyless) request: got %d, want 200", first.Code)
	}

	second := httptest.NewRecorder()
	h.ServeHTTP(second, anonRequest("203.0.113.71:51000"))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second (keyless) request: got %d, want 429 (metered normally)", second.Code)
	}

	// A wrong key does not exempt either.
	wrongKey := anonRequest("203.0.113.71:51000")
	wrongKey.Header.Set("X-Build-Key", "wrong")
	third := httptest.NewRecorder()
	h.ServeHTTP(third, wrongKey)
	if third.Code != http.StatusTooManyRequests {
		t.Fatalf("wrong-key request: got %d, want 429 (metered normally)", third.Code)
	}
}

func TestAnonRateLimit_DifferentIPsHaveIndependentBudgets(t *testing.T) {
	t.Parallel()

	clock := &fixedClock{t: time.Unix(2000, 0)}
	store := newAnonRateLimitStore(clock.now, 1)
	mw := AnonRateLimit(store, 1, nil, slogDiscard())
	h := mw(okHandler())

	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, anonRequest("203.0.113.20:1"))
	if rec1.Code != http.StatusOK {
		t.Fatalf("ip1 first request: got %d, want 200", rec1.Code)
	}

	rec1b := httptest.NewRecorder()
	h.ServeHTTP(rec1b, anonRequest("203.0.113.20:1"))
	if rec1b.Code != http.StatusTooManyRequests {
		t.Fatalf("ip1 second request: got %d, want 429 (budget exhausted)", rec1b.Code)
	}

	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, anonRequest("203.0.113.21:1"))
	if rec2.Code != http.StatusOK {
		t.Fatalf("ip2 first request: got %d, want 200 (independent budget)", rec2.Code)
	}
}

// TestAnonRateLimit_UnresolvableIPsShareOneConservativeBucket pins the spec
// decision for the "unresolvable client IP" case (issue #868 Phase 1):
// clientip.FromRequest returns the invalid zero netip.Addr only when
// RemoteAddr itself cannot be parsed (see the clientip package doc), and the
// zero value is identical for every such request regardless of the literal
// garbage that failed to parse — so they collapse onto one shared bucket with
// no special-casing required in this middleware.
func TestAnonRateLimit_UnresolvableIPsShareOneConservativeBucket(t *testing.T) {
	t.Parallel()

	clock := &fixedClock{t: time.Unix(2000, 0)}
	store := newAnonRateLimitStore(clock.now, 1)
	mw := AnonRateLimit(store, 1, nil, slogDiscard())
	h := mw(okHandler())

	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, anonRequest("garbage-peer-one"))
	if rec1.Code != http.StatusOK {
		t.Fatalf("first unresolvable request: got %d, want 200", rec1.Code)
	}

	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, anonRequest("garbage-peer-two"))
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second unresolvable request: got %d, want 429 (shared conservative bucket)", rec2.Code)
	}

	rec3 := httptest.NewRecorder()
	h.ServeHTTP(rec3, anonRequest("203.0.113.30:1"))
	if rec3.Code != http.StatusOK {
		t.Fatalf("resolved-ip request: got %d, want 200 (independent of the shared bucket)", rec3.Code)
	}
}

// TestAnonRateLimit_HealthCheckPathsExempt confirms GET /health and
// GET /v1/health never count against the anonymous budget — ACA's liveness
// and readiness probes hit these continuously, and metering them would waste
// budget a genuine anonymous caller could use.
func TestAnonRateLimit_HealthCheckPathsExempt(t *testing.T) {
	t.Parallel()

	clock := &fixedClock{t: time.Unix(2000, 0)}
	store := newAnonRateLimitStore(clock.now, 1)
	mw := AnonRateLimit(store, 1, nil, slogDiscard())
	h := mw(okHandler())

	for _, path := range []string{"/health", "/v1/health"} {
		for i := range 5 {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, anonRequestWithPath(path, "203.0.113.40:1", ""))
			if rec.Code != http.StatusOK {
				t.Fatalf("%s request %d: got %d, want 200 (health checks exempt)", path, i, rec.Code)
			}
			if got := rec.Header().Get("X-RateLimit-Limit"); got != "" {
				t.Errorf("%s request %d: got rate-limit header %q, want none (exempt)", path, i, got)
			}
		}
	}
}

// TestAnonRateLimit_PartialRefillRestoresPartialCapacity is the token-bucket
// behaviour that replaces the old fixed-window's all-or-nothing reset: after
// the burst is exhausted, a partial idle period restores only the tokens the
// refill rate actually earned, not the full bucket.
func TestAnonRateLimit_PartialRefillRestoresPartialCapacity(t *testing.T) {
	t.Parallel()

	clock := &fixedClock{t: time.Unix(2000, 0)}
	store := newAnonRateLimitStore(clock.now, 1) // 1 token/second refill
	mw := AnonRateLimit(store, 2, nil, slogDiscard())
	h := mw(okHandler())

	for range 2 {
		h.ServeHTTP(httptest.NewRecorder(), anonRequest("203.0.113.50:1"))
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, anonRequest("203.0.113.50:1"))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected throttle once burst is exhausted, got %d", rec.Code)
	}

	// Only 1 second passes: exactly 1 of the 2 burst tokens is earned back.
	clock.t = clock.t.Add(1 * time.Second)

	allowed := httptest.NewRecorder()
	h.ServeHTTP(allowed, anonRequest("203.0.113.50:1"))
	if allowed.Code != http.StatusOK {
		t.Fatalf("after partial refill: got %d, want 200 (1 token earned back)", allowed.Code)
	}

	// The single earned-back token was just spent; immediately trying again
	// (no further time elapsed) must throttle again.
	immediatelyAfter := httptest.NewRecorder()
	h.ServeHTTP(immediatelyAfter, anonRequest("203.0.113.50:1"))
	if immediatelyAfter.Code != http.StatusTooManyRequests {
		t.Fatalf("immediately after spending the earned token: got %d, want 429", immediatelyAfter.Code)
	}
}

// TestAnonRateLimit_FullIdleRestoresFullBurstCapacity confirms that idling
// long enough to earn back the whole burst behaves like a fresh bucket.
func TestAnonRateLimit_FullIdleRestoresFullBurstCapacity(t *testing.T) {
	t.Parallel()

	clock := &fixedClock{t: time.Unix(2000, 0)}
	store := newAnonRateLimitStore(clock.now, 1) // 1 token/second refill
	mw := AnonRateLimit(store, 2, nil, slogDiscard())
	h := mw(okHandler())

	for range 2 {
		h.ServeHTTP(httptest.NewRecorder(), anonRequest("203.0.113.51:1"))
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, anonRequest("203.0.113.51:1"))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected throttle before idling, got %d", rec.Code)
	}

	clock.t = clock.t.Add(61 * time.Second)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, anonRequest("203.0.113.51:1"))
	if rec2.Code != http.StatusOK {
		t.Errorf("after long idle: got %d, want 200", rec2.Code)
	}
	if got := rec2.Header().Get("X-RateLimit-Remaining"); got != "1" {
		t.Errorf("after long idle: X-RateLimit-Remaining got %q, want 1 (fresh bucket minus this request)", got)
	}
}

// TestAnonRateLimit_NoClientIPInLogs is the acceptance-criterion test for the
// clientip package's "never log, store, or export" constraint: it drives an
// allow then a throttle (the path most likely to want an observability log
// line) from a distinctive IP, then asserts that IP string never appears in
// any captured log record's message or attributes.
func TestAnonRateLimit_NoClientIPInLogs(t *testing.T) {
	t.Parallel()

	const distinctiveIP = "198.51.100.77"

	spy := &logSpy{}
	clock := &fixedClock{t: time.Unix(2000, 0)}
	store := newAnonRateLimitStore(clock.now, 1)
	mw := AnonRateLimit(store, 1, nil, slog.New(spy))
	h := mw(okHandler())

	h.ServeHTTP(httptest.NewRecorder(), anonRequest(distinctiveIP+":51000"))
	h.ServeHTTP(httptest.NewRecorder(), anonRequest(distinctiveIP+":51000"))

	spy.mu.Lock()
	defer spy.mu.Unlock()
	if len(spy.records) == 0 {
		t.Fatal("setup: expected at least one log record from the throttle path")
	}
	for _, r := range spy.records {
		if strings.Contains(r.Message, distinctiveIP) {
			t.Errorf("log message contains client IP: %q", r.Message)
		}
		r.Attrs(func(a slog.Attr) bool {
			if strings.Contains(a.Value.String(), distinctiveIP) {
				t.Errorf("log attr %s=%q contains client IP", a.Key, a.Value.String())
			}
			return true
		})
	}
}

// hasKey and tokensFor are test-only accessors used to assert eviction and
// refill behaviour without exposing them in production code.
func (s *anonRateLimitStore) hasKey(addr netip.Addr) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.buckets[addr]
	return ok
}

func (s *anonRateLimitStore) tokensFor(addr netip.Addr) float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buckets[addr].tokens
}

// TestAnonRateLimitStore_EvictsIdleIPKey is the GH#518 regression guard for the
// per-IP store, ported to token-bucket semantics: once an IP's bucket has
// fully refilled (idle long enough that it holds no information beyond "no
// entry"), the next checkAndIncrement call for that IP must reclaim the map
// key (delete) rather than leave a stale full bucket sitting in memory
// forever. burst=0 on the follow-up call forces the denied path (capacity 0
// clamps tokens to 0, which is also >= capacity so the delete fires) without
// re-adding an entry, making the deletion observable immediately.
func TestAnonRateLimitStore_EvictsIdleIPKey(t *testing.T) {
	t.Parallel()

	addr := netip.MustParseAddr("203.0.113.60")
	clock := &fixedClock{t: time.Unix(2000, 0)}
	store := newAnonRateLimitStore(clock.now, 1)

	store.checkAndIncrement(addr, 60)
	if !store.hasKey(addr) {
		t.Fatal("setup: expected key present after first request")
	}

	clock.t = clock.t.Add(61 * time.Second)

	store.checkAndIncrement(addr, 0)

	if store.hasKey(addr) {
		t.Error("expected map key deleted after bucket fully refilled, but key still present")
	}
}

// TestAnonRateLimitStore_MapStaysBoundedAcrossRefills is the acceptance-
// criterion test that the store's size shrinks/stays bounded after full
// refill, not merely "doesn't grow in the happy path": it seeds several IPs,
// lets each bucket fully refill, then drives a second wave of one request per
// IP — exactly what continuing real traffic looks like — and asserts the
// store neither accumulates stale per-IP state nor grows beyond the active
// population.
func TestAnonRateLimitStore_MapStaysBoundedAcrossRefills(t *testing.T) {
	t.Parallel()

	clock := &fixedClock{t: time.Unix(2000, 0)}
	store := newAnonRateLimitStore(clock.now, 1)

	addrs := []netip.Addr{
		netip.MustParseAddr("203.0.113.61"),
		netip.MustParseAddr("203.0.113.62"),
		netip.MustParseAddr("203.0.113.63"),
	}
	for _, a := range addrs {
		store.checkAndIncrement(a, 60)
	}
	if got := len(store.buckets); got != len(addrs) {
		t.Fatalf("setup: store size = %d, want %d", got, len(addrs))
	}

	// Advance well past full refill for every seeded bucket.
	clock.t = clock.t.Add(2 * time.Minute)

	for _, a := range addrs {
		store.checkAndIncrement(a, 60)
	}

	if got := len(store.buckets); got != len(addrs) {
		t.Errorf("store size after refill wave = %d, want %d (bounded, not accumulating)", got, len(addrs))
	}
	for _, a := range addrs {
		if got := store.tokensFor(a); got != 59 {
			t.Errorf("addr %v: tokens = %v, want 59 (fresh bucket minus the one request just made)", a, got)
		}
	}
}
