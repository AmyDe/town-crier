package planit

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

// recorderProvider returns an in-memory SDK TracerProvider for hermetic span
// assertions; the wrapped transport emits into the recorder, not the global.
func recorderProvider(t *testing.T) (*sdktrace.TracerProvider, *tracetest.SpanRecorder) {
	t.Helper()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	return tp, rec
}

func attr(attrs []attribute.KeyValue, key string) (attribute.Value, bool) {
	for _, a := range attrs {
		if string(a.Key) == key {
			return a.Value, true
		}
	}
	return attribute.Value{}, false
}

// TestFetchApplicationsPage_EmitsClientSpan asserts a PlanIt fetch produces a
// client span named "PlanIt search" carrying the upstream host as server.address
// so it surfaces in AppDependencies as a meaningful HTTP dependency.
func TestFetchApplicationsPage_EmitsClientSpan(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"total":0,"pg_sz":100,"records":[]}`))
	}))
	t.Cleanup(srv.Close)

	tp, rec := recorderProvider(t)

	c, err := NewClient(Options{
		BaseURL:      srv.URL,
		Throttle:     ThrottleOptions{DelayBetweenRequests: 0},
		Retry:        RetryOptions{MaxRetries: 0},
		HTTPClient:   &http.Client{Timeout: 5 * time.Second},
		Sleep:        func(context.Context, time.Duration) error { return nil },
		TraceOptions: []otelhttp.Option{otelhttp.WithTracerProvider(tp)},
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if _, err := c.FetchApplicationsPage(context.Background(), 99, nil, 1); err != nil {
		t.Fatalf("FetchApplicationsPage: %v", err)
	}

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 client span, got %d", len(spans))
	}
	span := spans[0]

	if span.Name() != "PlanIt search" {
		t.Errorf("span name: got %q, want %q", span.Name(), "PlanIt search")
	}
	if span.SpanKind() != oteltrace.SpanKindClient {
		t.Errorf("span kind: got %v, want client", span.SpanKind())
	}
	if _, ok := attr(span.Attributes(), "server.address"); !ok {
		t.Errorf("missing server.address attribute (Target source); attrs=%v", span.Attributes())
	}
}
