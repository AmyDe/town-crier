package platform

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// newRecorderProvider returns an in-memory SDK TracerProvider so transport tests
// are hermetic: the wrapped client emits spans into the recorder, never the
// global provider.
func newRecorderProvider(t *testing.T) (*sdktrace.TracerProvider, *tracetest.SpanRecorder) {
	t.Helper()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	return tp, rec
}

func attrLookupKV(attrs []attribute.KeyValue, key string) (attribute.Value, bool) {
	for _, a := range attrs {
		if string(a.Key) == key {
			return a.Value, true
		}
	}
	return attribute.Value{}, false
}

// TestWrapHTTPClient_EmitsNamedClientSpan asserts the wrapped client's transport
// produces a client span whose name comes from the supplied formatter and whose
// server.address attribute carries the upstream host, so AppDependencies renders
// it as an HTTP dependency with a meaningful Target + Name.
func TestWrapHTTPClient_EmitsNamedClientSpan(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	tp, rec := newRecorderProvider(t)

	base := &http.Client{Timeout: 7 * time.Second}
	formatter := func(string, *http.Request) string { return "TestService op" }
	wrapped := WrapHTTPClient(base, formatter, otelhttp.WithTracerProvider(tp))

	if wrapped.Timeout != 7*time.Second {
		t.Fatalf("WrapHTTPClient must preserve Timeout: got %s, want 7s", wrapped.Timeout)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, http.NoBody)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := wrapped.Do(req)
	if err != nil {
		t.Fatalf("wrapped.Do: %v", err)
	}
	_ = resp.Body.Close()

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 client span, got %d", len(spans))
	}
	span := spans[0]

	if span.Name() != "TestService op" {
		t.Errorf("span name: got %q, want %q", span.Name(), "TestService op")
	}
	if span.SpanKind() != oteltrace.SpanKindClient {
		t.Errorf("span kind: got %v, want client", span.SpanKind())
	}

	wantHost, _, _ := splitHostPortTest(srv.URL)
	got, ok := attrLookupKV(span.Attributes(), "server.address")
	if !ok {
		t.Fatalf("missing server.address attribute (Target source); attrs=%v", span.Attributes())
	}
	if got.AsString() != wantHost {
		t.Errorf("server.address: got %q, want %q", got.AsString(), wantHost)
	}
}

// TestWrapHTTPClient_NilTransportUsesDefault asserts a client with no explicit
// transport is wrapped against http.DefaultTransport (otelhttp's documented
// fallback), so the call still succeeds and produces a span.
func TestWrapHTTPClient_NilTransportUsesDefault(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	tp, rec := newRecorderProvider(t)

	base := &http.Client{} // no Transport set
	wrapped := WrapHTTPClient(base, func(string, *http.Request) string { return "Default op" }, otelhttp.WithTracerProvider(tp))

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, http.NoBody)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := wrapped.Do(req)
	if err != nil {
		t.Fatalf("wrapped.Do: %v", err)
	}
	_ = resp.Body.Close()

	if got := len(rec.Ended()); got != 1 {
		t.Fatalf("expected 1 span via DefaultTransport, got %d", got)
	}
}

// splitHostPortTest pulls the host out of an httptest URL (http://127.0.0.1:NNNN)
// for the server.address assertion.
func splitHostPortTest(rawURL string) (host, port string, ok bool) {
	const prefix = "http://"
	hp := rawURL[len(prefix):]
	for i := 0; i < len(hp); i++ {
		if hp[i] == ':' {
			return hp[:i], hp[i+1:], true
		}
	}
	return hp, "", false
}
