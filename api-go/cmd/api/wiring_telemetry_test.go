package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"go.opentelemetry.io/otel"
)

// recordSpansFor drives method+path through the full newRouter chain (otelhttp
// outermost) with a recording TracerProvider installed globally, and returns the
// recorded request spans. The router runs deny-all + Cosmos-less, so it exercises
// only routes that need no store.
func recordSpansFor(t *testing.T, h http.Handler, method, path, authz string) []sdktrace.ReadOnlySpan {
	t.Helper()

	prev := otel.GetTracerProvider()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(prev)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req := httptest.NewRequestWithContext(ctx, method, path, nil)
	if authz != "" {
		req.Header.Set("Authorization", authz)
	}
	h.ServeHTTP(httptest.NewRecorder(), req)
	return rec.Ended()
}

// TestRouter_RequestSpanCarriesRouteAndStatus pins tc-r8eo end to end: a request
// through the production chain produces a request span whose Name is the matched
// route pattern and which carries http.route and http.response.status_code, so
// Azure Monitor's AppRequests.Name and ResultCode are set correctly (tc-r8eo).
func TestRouter_RequestSpanCarriesRouteAndStatus(t *testing.T) {
	for _, tc := range []struct {
		name         string
		method       string
		path         string
		authz        string
		wantRoute    string
		wantStatus   int64
		wantHasRoute bool
	}{
		// Anonymous matched route, 200.
		{"health 200", http.MethodGet, "/v1/health", "", "GET /v1/health", http.StatusOK, true},
		// Authed matched route with no token -> the fallback-deny 401. The route
		// still matched (geocode is always wired, even Cosmos-less), so the span is
		// named after it.
		{"geocode 401", http.MethodGet, "/v1/geocode/SW1A1AA", "", "GET /v1/geocode/{postcode}", http.StatusUnauthorized, true},
		// Unmatched path -> 401 fallback, no route, default span name preserved.
		{"unmatched 401", http.MethodGet, "/v1/nope", "", "", http.StatusUnauthorized, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHandler(t)
			spans := recordSpansFor(t, h, tc.method, tc.path, tc.authz)

			var req sdktrace.ReadOnlySpan
			for _, s := range spans {
				if s.SpanKind().String() == "server" {
					req = s
				}
			}
			if req == nil {
				t.Fatalf("no server span recorded (got %d spans)", len(spans))
			}

			gotStatus, ok := attrInt64(req, "http.response.status_code")
			if !ok {
				t.Fatalf("http.response.status_code attribute missing")
			}
			if gotStatus != tc.wantStatus {
				t.Errorf("http.response.status_code = %d, want %d", gotStatus, tc.wantStatus)
			}

			gotRoute := attrStr(req, "http.route")
			if tc.wantHasRoute {
				if gotRoute != tc.wantRoute {
					t.Errorf("http.route = %q, want %q", gotRoute, tc.wantRoute)
				}
				if got := req.Name(); got != tc.wantRoute {
					t.Errorf("span name = %q, want %q", got, tc.wantRoute)
				}
			} else if gotRoute != "" {
				t.Errorf("http.route = %q, want empty for unmatched request", gotRoute)
			}
		})
	}
}

func attrInt64(span sdktrace.ReadOnlySpan, key string) (int64, bool) {
	for _, kv := range span.Attributes() {
		if string(kv.Key) == key {
			return kv.Value.AsInt64(), true
		}
	}
	return 0, false
}

func attrStr(span sdktrace.ReadOnlySpan, key string) string {
	for _, kv := range span.Attributes() {
		if string(kv.Key) == key {
			return kv.Value.AsString()
		}
	}
	return ""
}
