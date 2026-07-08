package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// anonRateLimitRequest builds a GET request with no Authorization header and a
// caller-chosen RemoteAddr, driving the full router chain's anonymous path.
func anonRateLimitRequest(t *testing.T, h http.Handler, path, remoteAddr string) *httptest.ResponseRecorder {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	req.RemoteAddr = remoteAddr
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// newRouterWithAnonLimit builds a minimal store-less router with a
// caller-chosen anonymous rate-limit budget, so tests can drive the limiter
// past its threshold without waiting out the production 60-request default.
func newRouterWithAnonLimit(t *testing.T, logger *slog.Logger, anonRequests, anonWindowSeconds int) http.Handler {
	t.Helper()
	return newRouter(denyAllValidator{}, []string{"https://towncrierapp.uk"}, nil, profiles.NoOpAuth0Client{}, profiles.CascadeDeleters{}, profiles.ExportReaders{}, nil, nil, nil, nil, nil, nil, testGeocodeClient(t), testDesignationClient(t), nil, nil, "", "", nil, nil, "", nil, nil, nil, anonRequests, anonWindowSeconds, logger)
}

// TestRouter_AnonRateLimitAppliesToAnonymousRoutes proves AnonRateLimit is
// actually wired into the production dispatch chain (not just unit tested in
// isolation, GH#868 Phase 1): with a tiny per-router budget, a real anonymous
// route throttles once that budget is exhausted.
func TestRouter_AnonRateLimitAppliesToAnonymousRoutes(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	h := newRouterWithAnonLimit(t, logger, 2, 60)

	for i := range 2 {
		rec := anonRateLimitRequest(t, h, "/v1/version-config", "203.0.113.90:1")
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: got %d, want 200", i, rec.Code)
		}
	}

	rec := anonRateLimitRequest(t, h, "/v1/version-config", "203.0.113.90:1")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("3rd request: got %d, want 429", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got == "" {
		t.Error("429 missing Retry-After")
	}
}

// TestRouter_AnonRateLimitHealthChecksExemptEvenWithTinyBudget confirms the
// health-check exemption survives the real wiring: with a limit of 1, five
// consecutive /v1/health hits from the same IP all succeed.
func TestRouter_AnonRateLimitHealthChecksExemptEvenWithTinyBudget(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	h := newRouterWithAnonLimit(t, logger, 1, 60)

	for i := range 5 {
		rec := anonRateLimitRequest(t, h, "/v1/health", "203.0.113.91:1")
		if rec.Code != http.StatusOK {
			t.Fatalf("health request %d: got %d, want 200", i, rec.Code)
		}
	}
}

// TestRouter_AnonRateLimitDoesNotThrottleAuthenticatedTraffic confirms that,
// wired end to end, an authenticated caller sharing an IP with an exhausted
// anonymous budget is unaffected — the anonymous limiter must never touch a
// request carrying a subject.
func TestRouter_AnonRateLimitDoesNotThrottleAuthenticatedTraffic(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	validator := staticValidator{claims: auth.Claims{Subject: "auth0|anonwire"}}
	h := newRouter(validator, []string{"https://towncrierapp.uk"}, nil, profiles.NoOpAuth0Client{}, profiles.CascadeDeleters{}, profiles.ExportReaders{}, nil, nil, nil, nil, nil, nil, testGeocodeClient(t), testDesignationClient(t), nil, nil, "", "", nil, nil, "", nil, nil, nil, 1, 60, logger)

	const sameIP = "203.0.113.92:1"

	first := anonRateLimitRequest(t, h, "/v1/version-config", sameIP)
	if first.Code != http.StatusOK {
		t.Fatalf("first anonymous request: got %d, want 200", first.Code)
	}
	second := anonRateLimitRequest(t, h, "/v1/version-config", sameIP)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second anonymous request: got %d, want 429 (budget exhausted)", second.Code)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/geocode/SW1A1AA", nil)
	req.RemoteAddr = sameIP
	req.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code == http.StatusTooManyRequests {
		t.Fatalf("authenticated request throttled by the anonymous limiter: got %d", rec.Code)
	}
}
