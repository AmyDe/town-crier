package middleware

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// TestStripForwardedFor_RemovesHeaderBeforeNextHandler confirms the header is
// gone by the time the wrapped handler runs, for both the single-hop and the
// Cloudflare-style appended list.
func TestStripForwardedFor_RemovesHeaderBeforeNextHandler(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
	}{
		{"single hop", "82.132.213.185"},
		{"appended list", "82.132.213.185, 172.70.42.11"},
		{"ipv6", "2a00:23c8:8ab:801:7026:db85:2b2b:9aee"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			var present bool
			next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = r.Header.Get("X-Forwarded-For")
				_, present = r.Header["X-Forwarded-For"]
			})

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/health", nil)
			req.Header.Set("X-Forwarded-For", tc.value)
			StripForwardedFor(next).ServeHTTP(httptest.NewRecorder(), req)

			if present {
				t.Errorf("X-Forwarded-For still present downstream with value %q", got)
			}
		})
	}
}

// TestStripForwardedFor_LeavesOtherHeadersAlone guards the blast radius: only
// X-Forwarded-For goes. CF-Connecting-IP in particular must survive, because
// internal/clientip reads it (validated against Cloudflare ranges) for the
// anonymous rate limiter.
func TestStripForwardedFor_LeavesOtherHeadersAlone(t *testing.T) {
	var seen http.Header
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = r.Header.Clone()
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/health", nil)
	req.Header.Set("X-Forwarded-For", "82.132.213.185")
	req.Header.Set("CF-Connecting-IP", "82.132.213.185")
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("User-Agent", "TownCrierApp/64 CFNetwork/3860.600.12 Darwin/25.5.0")
	StripForwardedFor(next).ServeHTTP(httptest.NewRecorder(), req)

	for _, key := range []string{"Cf-Connecting-Ip", "Authorization", "User-Agent"} {
		if seen.Get(key) == "" {
			t.Errorf("%s was dropped, want it preserved", key)
		}
	}
}

// TestStripForwardedFor_NoHeaderIsANoOp confirms a request that never carried
// the header passes through untouched.
func TestStripForwardedFor_NoHeaderIsANoOp(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	StripForwardedFor(next).ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/health", nil))

	if !called {
		t.Error("next handler was not called")
	}
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

// serveThroughOtelHTTP drives a GET through StripForwardedFor -> otelhttp (the
// production nesting from cmd/api/wiring.go) with the given X-Forwarded-For and
// returns the recorded server span.
func serveThroughOtelHTTP(t *testing.T, forwardedFor string) sdktrace.ReadOnlySpan {
	t.Helper()

	prev := otel.GetTracerProvider()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(prev)
	})

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(StripForwardedFor(otelhttp.NewHandler(inner, "town-crier-api-go")))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/v1/health", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if forwardedFor != "" {
		req.Header.Set("X-Forwarded-For", forwardedFor)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	return spans[0]
}

// TestStripForwardedFor_SpanNeverCarriesTheForwardedIP is the regression test
// that matters: it pins the OUTCOME rather than the mechanism. Real UK
// residential IPs were reaching App Insights AppRequests through the
// client.address attribute (30-day retention), against the project's
// no-client-IP-logging stance.
//
// otelhttp resolves client.address as options -> X-Forwarded-For -> peer
// address, so removing the header does not delete the attribute; it demotes it
// to the TCP peer, which behind Cloudflare and Container Apps ingress is the
// ingress address, not a person. What must never happen is a forwarded value
// surviving, and that is what this asserts.
//
// If a future otelhttp bump starts sourcing the client IP from somewhere else
// (X-Real-IP, Forwarded), this test fails and the strip needs widening.
func TestStripForwardedFor_SpanNeverCarriesTheForwardedIP(t *testing.T) {
	const userIP = "82.132.213.185"
	span := serveThroughOtelHTTP(t, userIP+", 172.70.42.11")

	clientAddr := attrString(span, "client.address")
	if clientAddr == userIP {
		t.Errorf("client.address = %q — the forwarded end-user IP reached the span", clientAddr)
	}
	// Whatever survives must be the TCP peer, i.e. infrastructure. Pinning the
	// equality (rather than just "not the user IP") is what catches a future
	// otelhttp picking up some other client-supplied header.
	if peer := attrString(span, "network.peer.address"); clientAddr != peer {
		t.Errorf("client.address = %q, want it to equal network.peer.address %q", clientAddr, peer)
	}
}

// TestStripForwardedFor_KeepsUsefulSpanAttributes confirms the strip is
// surgical: the non-PII request attributes the dashboards and the web-vs-iOS
// client split depend on still reach the span.
func TestStripForwardedFor_KeepsUsefulSpanAttributes(t *testing.T) {
	span := serveThroughOtelHTTP(t, "82.132.213.185")

	for _, key := range []string{"http.request.method", "url.path", "user_agent.original"} {
		if attrString(span, key) == "" {
			t.Errorf("%s missing from the span, want it preserved", key)
		}
	}
}
