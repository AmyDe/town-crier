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
	store := newAnonRateLimitStore(clock.now, time.Minute)
	mw := AnonRateLimit(store, 60, slogDiscard())

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

func TestAnonRateLimit_RequestsBelowLimitAreUnaffected(t *testing.T) {
	t.Parallel()

	clock := &fixedClock{t: time.Unix(2000, 0)}
	store := newAnonRateLimitStore(clock.now, time.Minute)
	mw := AnonRateLimit(store, 5, slogDiscard())
	h := mw(okHandler())

	for i := range 5 {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, anonRequest("203.0.113.12:51000"))
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: got %d, want 200 (below limit)", i, rec.Code)
		}
	}
}

func TestAnonRateLimit_ExceededReturns429WithHeaders(t *testing.T) {
	t.Parallel()

	clock := &fixedClock{t: time.Unix(2000, 0)}
	store := newAnonRateLimitStore(clock.now, time.Minute)
	mw := AnonRateLimit(store, 60, slogDiscard())
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
// has exhausted the (deliberately tiny, limit=1) anonymous budget.
func TestAnonRateLimit_AuthenticatedRequestPassesThrough(t *testing.T) {
	t.Parallel()

	clock := &fixedClock{t: time.Unix(2000, 0)}
	store := newAnonRateLimitStore(clock.now, time.Minute)
	mw := AnonRateLimit(store, 1, slogDiscard())
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
	// IP's counter.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, anonRequest("203.0.113.13:51000"))
	if rec.Code != http.StatusOK {
		t.Fatalf("first anonymous request on the IP: got %d, want 200", rec.Code)
	}
}

func TestAnonRateLimit_DifferentIPsHaveIndependentBudgets(t *testing.T) {
	t.Parallel()

	clock := &fixedClock{t: time.Unix(2000, 0)}
	store := newAnonRateLimitStore(clock.now, time.Minute)
	mw := AnonRateLimit(store, 1, slogDiscard())
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
	store := newAnonRateLimitStore(clock.now, time.Minute)
	mw := AnonRateLimit(store, 1, slogDiscard())
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
	store := newAnonRateLimitStore(clock.now, time.Minute)
	mw := AnonRateLimit(store, 1, slogDiscard())
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

func TestAnonRateLimit_WindowResets(t *testing.T) {
	t.Parallel()

	clock := &fixedClock{t: time.Unix(2000, 0)}
	store := newAnonRateLimitStore(clock.now, time.Minute)
	mw := AnonRateLimit(store, 2, slogDiscard())
	h := mw(okHandler())

	for range 2 {
		h.ServeHTTP(httptest.NewRecorder(), anonRequest("203.0.113.50:1"))
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, anonRequest("203.0.113.50:1"))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected throttle before window reset, got %d", rec.Code)
	}

	clock.t = clock.t.Add(61 * time.Second)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, anonRequest("203.0.113.50:1"))
	if rec2.Code != http.StatusOK {
		t.Errorf("after window reset: got %d, want 200", rec2.Code)
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
	store := newAnonRateLimitStore(clock.now, time.Minute)
	mw := AnonRateLimit(store, 1, slog.New(spy))
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

// hasKey and keyLen are test-only accessors mirroring rateLimitStore's (see
// ratelimit_test.go), used to assert eviction behaviour without exposing them
// in production code.
func (s *anonRateLimitStore) hasKey(addr netip.Addr) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.requests[addr]
	return ok
}

func (s *anonRateLimitStore) keyLen(addr netip.Addr) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.requests[addr])
}

// TestAnonRateLimitStore_EvictsIdleIPKey is the GH#518 regression guard for the
// per-IP store: once an IP's in-window timestamps have all aged out, the next
// checkAndIncrement call for that IP must reclaim the map key (delete) rather
// than leave a stale entry sitting in memory forever.
func TestAnonRateLimitStore_EvictsIdleIPKey(t *testing.T) {
	t.Parallel()

	addr := netip.MustParseAddr("203.0.113.60")
	clock := &fixedClock{t: time.Unix(2000, 0)}
	store := newAnonRateLimitStore(clock.now, time.Minute)

	store.checkAndIncrement(addr, 60)
	if !store.hasKey(addr) {
		t.Fatal("setup: expected key present after first request")
	}

	clock.t = clock.t.Add(61 * time.Second)

	// limit=0 forces the denied path with kept==0 in a single call, making the
	// deletion observable immediately (mirrors TestRateLimitStore_EvictsIdleUserKey).
	store.checkAndIncrement(addr, 0)

	if store.hasKey(addr) {
		t.Error("expected map key deleted after all timestamps aged out, but key still present")
	}
}

// TestAnonRateLimitStore_MapStaysBoundedAcrossWindows is the acceptance-
// criterion test that the store's size shrinks/stays bounded after window
// expiry, not merely "doesn't grow in the happy path": it seeds several IPs,
// lets the window expire, then drives a second wave of one request per IP —
// exactly what continuing real traffic looks like — and asserts the store
// neither accumulates a second stale timestamp per IP nor grows beyond the
// active population.
func TestAnonRateLimitStore_MapStaysBoundedAcrossWindows(t *testing.T) {
	t.Parallel()

	clock := &fixedClock{t: time.Unix(2000, 0)}
	store := newAnonRateLimitStore(clock.now, time.Minute)

	addrs := []netip.Addr{
		netip.MustParseAddr("203.0.113.61"),
		netip.MustParseAddr("203.0.113.62"),
		netip.MustParseAddr("203.0.113.63"),
	}
	for _, a := range addrs {
		store.checkAndIncrement(a, 60)
	}
	if got := len(store.requests); got != len(addrs) {
		t.Fatalf("setup: store size = %d, want %d", got, len(addrs))
	}

	// Advance past the window: every seeded timestamp is now stale.
	clock.t = clock.t.Add(2 * time.Minute)

	for _, a := range addrs {
		store.checkAndIncrement(a, 60)
	}

	if got := len(store.requests); got != len(addrs) {
		t.Errorf("store size after second window = %d, want %d (bounded, not accumulating)", got, len(addrs))
	}
	for _, a := range addrs {
		if got := store.keyLen(a); got != 1 {
			t.Errorf("addr %v: stored len = %d, want 1 (stale timestamp evicted before recording the new one)", a, got)
		}
	}
}
