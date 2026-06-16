package middleware

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// attrString returns the string value of the named attribute on the span, or ""
// if the span carries no such attribute.
func attrString(span sdktrace.ReadOnlySpan, key string) string {
	for _, kv := range span.Attributes() {
		if string(kv.Key) == key {
			return kv.Value.AsString()
		}
	}
	return ""
}

// attrInt returns the int64 value of the named attribute on the span and whether
// it was present.
func attrInt(span sdktrace.ReadOnlySpan, key string) (int64, bool) {
	for _, kv := range span.Attributes() {
		if string(kv.Key) == key {
			return kv.Value.AsInt64(), true
		}
	}
	return 0, false
}

// serveRouteSpanChain drives a GET request for path through
// spanStarter -> RouteSpan(mux) -> mux and returns the single recorded request
// span. The mux registers a single GET pattern that responds with status.
func serveRouteSpanChain(t *testing.T, pattern, path string, status int) sdktrace.ReadOnlySpan {
	t.Helper()

	prev := otel.GetTracerProvider()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(prev)
	})

	mux := http.NewServeMux()
	if pattern != "" {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
		})
	}

	chain := spanStarter(RouteSpan(mux)(mux))
	srv := httptest.NewServer(chain)
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+path, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
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

// TestRouteSpan_NamesSpanAfterMatchedRoute pins tc-r8eo: a matched route makes
// the request span Name the route pattern (e.g. "GET /v1/health") and carries
// the http.route attribute, so Azure Monitor's AppRequests.Name shows the
// endpoint rather than the bare verb.
func TestRouteSpan_NamesSpanAfterMatchedRoute(t *testing.T) {
	span := serveRouteSpanChain(t, "GET /v1/health", "/v1/health", http.StatusOK)

	if got, want := span.Name(), "GET /v1/health"; got != want {
		t.Errorf("span name = %q, want %q", got, want)
	}
	if got, want := attrString(span, string(attribute.Key("http.route"))), "GET /v1/health"; got != want {
		t.Errorf("http.route = %q, want %q", got, want)
	}
}

// TestRouteSpan_RecordsStatusCode confirms the real HTTP status is recorded onto
// the span as http.response.status_code for both a 2xx and an error path, so
// Azure Monitor's ResultCode reflects the HTTP status rather than the span code.
func TestRouteSpan_RecordsStatusCode(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
	}{
		{"ok", http.StatusOK},
		{"unauthorized", http.StatusUnauthorized},
		{"not found", http.StatusNotFound},
		{"server error", http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			span := serveRouteSpanChain(t, "GET /v1/thing", "/v1/thing", tc.status)
			got, ok := attrInt(span, "http.response.status_code")
			if !ok {
				t.Fatalf("http.response.status_code attribute missing")
			}
			if int(got) != tc.status {
				t.Errorf("http.response.status_code = %d, want %d", got, tc.status)
			}
		})
	}
}

// TestRouteSpan_UnmatchedRequestFallsBackGracefully confirms an unmatched
// request neither panics nor renames the span to an empty string: the default
// span name survives and no http.route attribute is set, while the status code
// (the mux's own 404) is still recorded.
func TestRouteSpan_UnmatchedRequestFallsBackGracefully(t *testing.T) {
	// No pattern registered: every path is unmatched -> the mux's default 404.
	span := serveRouteSpanChain(t, "", "/nope", http.StatusNotFound)

	if got := span.Name(); got != "request" {
		t.Errorf("span name = %q, want the default %q (unmatched must not rename)", got, "request")
	}
	if got := attrString(span, "http.route"); got != "" {
		t.Errorf("http.route = %q, want empty for an unmatched request", got)
	}
	if got, ok := attrInt(span, "http.response.status_code"); !ok || int(got) != http.StatusNotFound {
		t.Errorf("http.response.status_code = %d (ok=%v), want 404", got, ok)
	}
}
