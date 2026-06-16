package apns

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

// TestSendOne_EmitsClientSpan asserts an APNs push produces a client span named
// "APNs push" carrying the upstream host as server.address so it surfaces in
// AppDependencies as a meaningful HTTP dependency. It exercises the same
// platform.WrapHTTPClient wiring NewClient applies in production.
func TestSendOne_EmitsClientSpan(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	tp, rec := recorderProvider(t)

	pemBytes, _ := newTestKeyPEM(t)
	opts := Options{
		Enabled:  true,
		AuthKey:  string(pemBytes),
		KeyID:    "L2J5PQASN5",
		TeamID:   "4574VQ7N2X",
		BundleID: "uk.towncrierapp.mobile",
	}
	traced := platform.WrapHTTPClient(
		srv.Client(),
		func(string, *http.Request) string { return "APNs push" },
		otelhttp.WithTracerProvider(tp),
	)
	client, err := newClientWithBaseURL(opts, srv.URL, traced, testLogger(), func() time.Time {
		return time.Unix(1_700_000_000, 0).UTC()
	})
	if err != nil {
		t.Fatalf("newClientWithBaseURL: %v", err)
	}

	if _, err := client.Send(context.Background(), []string{"device-token-abc123"}, []byte(`{"aps":{}}`)); err != nil {
		t.Fatalf("Send: %v", err)
	}

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 client span, got %d", len(spans))
	}
	span := spans[0]

	if span.Name() != "APNs push" {
		t.Errorf("span name: got %q, want %q", span.Name(), "APNs push")
	}
	if span.SpanKind() != oteltrace.SpanKindClient {
		t.Errorf("span kind: got %v, want client", span.SpanKind())
	}
	if _, ok := spanAttr(span.Attributes(), "server.address"); !ok {
		t.Errorf("missing server.address attribute (Target source); attrs=%v", span.Attributes())
	}
}
