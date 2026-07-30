package fcm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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

// sendSpan picks out the FCM send span (as opposed to the OAuth token-exchange
// span, which shares the same "FCM push" formatter name since both requests
// go through the same traced httpClient) by its url.full containing the FCM
// v1 messages:send path.
func sendSpan(t *testing.T, spans []sdktrace.ReadOnlySpan) sdktrace.ReadOnlySpan {
	t.Helper()
	for _, s := range spans {
		if v, ok := spanAttr(s.Attributes(), "url.full"); ok && strings.Contains(v.AsString(), "messages:send") {
			return s
		}
	}
	t.Fatalf("no FCM send span found among %d spans", len(spans))
	return nil
}

// TestSendOne_EmitsClientSpan asserts an FCM push produces a client span named
// "FCM push" carrying the upstream host as server.address, mirroring
// internal/apns's span coverage. Two client spans are emitted per Send — the
// OAuth token exchange and the FCM send itself — since both share the traced
// httpClient; this test asserts on the FCM send span.
func TestSendOne_EmitsClientSpan(t *testing.T) {
	t.Parallel()

	fcmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(fcmSrv.Close)
	tokenSrv, _ := tokenServer(t)

	tp, rec := recorderProvider(t)

	pemBytes, _ := newTestKeyPEM(t)
	opts := Options{
		Enabled:            true,
		ProjectID:          "town-crier-test",
		ServiceAccountJSON: newTestServiceAccountJSON(t, tokenSrv.URL, pemBytes),
	}
	traced := platform.WrapHTTPClient(
		&http.Client{},
		func(string, *http.Request) string { return "FCM push" },
		otelhttp.WithTracerProvider(tp),
	)
	client, err := newClientWithBaseURL(opts, fcmSrv.URL, traced, testLogger(), fixedClock(1_700_000_000))
	if err != nil {
		t.Fatalf("newClientWithBaseURL: %v", err)
	}

	payload := []byte(`{"notification":{"title":"hi","body":"there"}}`)
	if _, err := client.Send(context.Background(), []string{"device-token-abc123"}, payload); err != nil {
		t.Fatalf("Send: %v", err)
	}

	span := sendSpan(t, rec.Ended())

	if span.Name() != "FCM push" {
		t.Errorf("span name: got %q, want %q", span.Name(), "FCM push")
	}
	if span.SpanKind() != oteltrace.SpanKindClient {
		t.Errorf("span kind: got %v, want client", span.SpanKind())
	}
	if _, ok := spanAttr(span.Attributes(), "server.address"); !ok {
		t.Errorf("missing server.address attribute (Target source); attrs=%v", span.Attributes())
	}
}

// TestSendOne_SpanURLDoesNotCarryDeviceToken is a regression guard: the FCM v1
// send URL is /v1/projects/<projectID>/messages:send — the device token only
// ever goes into the JSON body (see withToken), never the URL path — so no
// client span's url.full attribute (FCM send or OAuth token exchange) should
// ever carry it. Unlike APNs, no platform.RedactingTransport wiring is needed
// here because there is nothing in the URL to redact; this test exists to
// catch a future regression if the URL shape ever changes to embed the token.
func TestSendOne_SpanURLDoesNotCarryDeviceToken(t *testing.T) {
	t.Parallel()

	fcmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(fcmSrv.Close)
	tokenSrv, _ := tokenServer(t)

	tp, rec := recorderProvider(t)

	pemBytes, _ := newTestKeyPEM(t)
	opts := Options{
		Enabled:            true,
		ProjectID:          "town-crier-test",
		ServiceAccountJSON: newTestServiceAccountJSON(t, tokenSrv.URL, pemBytes),
	}
	traced := platform.WrapHTTPClient(
		&http.Client{},
		func(string, *http.Request) string { return "FCM push" },
		otelhttp.WithTracerProvider(tp),
	)
	client, err := newClientWithBaseURL(opts, fcmSrv.URL, traced, testLogger(), fixedClock(1_700_000_000))
	if err != nil {
		t.Fatalf("newClientWithBaseURL: %v", err)
	}

	const token = "device-token-abc123"
	payload := []byte(`{"notification":{"title":"hi","body":"there"}}`)
	if _, err := client.Send(context.Background(), []string{token}, payload); err != nil {
		t.Fatalf("Send: %v", err)
	}

	spans := rec.Ended()
	if len(spans) == 0 {
		t.Fatalf("expected at least 1 client span, got 0")
	}
	for _, span := range spans {
		got, ok := spanAttr(span.Attributes(), "url.full")
		if !ok {
			t.Fatalf("missing url.full attribute on span %q; attrs=%v", span.Name(), span.Attributes())
		}
		if strings.Contains(got.AsString(), token) {
			t.Errorf("span %q url.full leaked the raw device token: %q", span.Name(), got.AsString())
		}
	}
}
