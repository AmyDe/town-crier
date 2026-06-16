package profiles

import (
	"context"
	"net/http"
	"testing"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
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

func spanAttr(attrs []attribute.KeyValue, key string) (attribute.Value, bool) {
	for _, a := range attrs {
		if string(a.Key) == key {
			return a.Value, true
		}
	}
	return attribute.Value{}, false
}

// TestAuth0Client_EmitsClientSpan asserts an Auth0 Management call produces
// client spans named "Auth0 token" carrying the upstream host as server.address,
// so token/PATCH/DELETE traffic surfaces in AppDependencies as a meaningful HTTP
// dependency. It exercises the same platform.WrapHTTPClient wiring the api/worker
// main() call sites apply in production. The static span name keeps cardinality
// low (no per-user subject in the name).
func TestAuth0Client_EmitsClientSpan(t *testing.T) {
	t.Parallel()

	_, srv := newAuth0Server(t)
	tp, rec := recorderProvider(t)

	traced := platform.WrapHTTPClient(
		srv.Client(),
		func(string, *http.Request) string { return "Auth0 token" },
		otelhttp.WithTracerProvider(tp),
	)
	client := NewAuth0Client(traced, srv.URL, "client-id", platform.NewSecret("client-secret"))

	if err := client.UpdateSubscriptionTier(context.Background(), "auth0|user-1", "premium"); err != nil {
		t.Fatalf("UpdateSubscriptionTier: %v", err)
	}

	spans := rec.Ended()
	if len(spans) == 0 {
		t.Fatalf("expected at least 1 client span, got 0")
	}
	for _, span := range spans {
		if span.Name() != "Auth0 token" {
			t.Errorf("span name: got %q, want %q", span.Name(), "Auth0 token")
		}
		if span.SpanKind() != oteltrace.SpanKindClient {
			t.Errorf("span kind: got %v, want client", span.SpanKind())
		}
		if _, ok := spanAttr(span.Attributes(), "server.address"); !ok {
			t.Errorf("missing server.address attribute (Target source); attrs=%v", span.Attributes())
		}
	}
}
