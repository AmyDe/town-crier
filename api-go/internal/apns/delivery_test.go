package apns

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// fakePushMetrics is a hand-written double for pushMetricsRecorder, recording
// every platform tag the client reports a delivery failure for.
type fakePushMetrics struct {
	platforms []string
}

func (f *fakePushMetrics) PushDeliveryFailed(_ context.Context, platform string) {
	f.platforms = append(f.platforms, platform)
}

// recordDeliverySpans swaps in an in-memory SDK TracerProvider for the
// duration of run, restoring the previous global provider on cleanup, and
// returns every span recorded. Deliberately not t.Parallel(): mutating the
// global TracerProvider is safe only while no sibling test's body is
// concurrently executing (matches internal/acsemail/instrumented_test.go and
// this package's own span_test.go, which use a locally-scoped provider
// instead since the HTTP-transport span is wired explicitly per test).
func recordDeliverySpans(t *testing.T, run func()) []sdktrace.ReadOnlySpan {
	t.Helper()

	prev := otel.GetTracerProvider()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(prev)
	})

	run()

	return rec.Ended()
}

// TestClient_Send_RecordsDeliveryFailureSpanAndMetric pins tc-97k35.4: a
// genuine per-device delivery failure (here, the APNs host is unreachable, so
// sendOne exhausts its retries before ever getting a response) must produce
// an "APNs delivery" span with Error status plus a
// towncrier.push.delivery_failed(platform=apns) count — not just the
// "apns send failed" slog line, which is all today's code emits.
func TestClient_Send_RecordsDeliveryFailureSpanAndMetric(t *testing.T) {
	pemBytes, _ := newTestKeyPEM(t)
	opts := Options{
		Enabled:  true,
		AuthKey:  string(pemBytes),
		KeyID:    "L2J5PQASN5",
		TeamID:   "4574VQ7N2X",
		BundleID: "uk.towncrierapp.mobile",
	}
	// 127.0.0.1:1 is a privileged, unlistened port: every connection attempt
	// fails immediately with "connection refused", exercising sendOne's
	// error path (exhausted transport retries) without needing to race an
	// httptest server's shutdown.
	client, err := newClientWithBaseURL(opts, "http://127.0.0.1:1", &http.Client{}, testLogger(), func() time.Time {
		return time.Unix(1_700_000_000, 0).UTC()
	})
	if err != nil {
		t.Fatalf("newClientWithBaseURL: %v", err)
	}
	metricsRec := &fakePushMetrics{}
	client.WithMetrics(metricsRec)

	spans := recordDeliverySpans(t, func() {
		invalid, sendErr := client.Send(context.Background(), []string{"tok"}, json.RawMessage(`{}`))
		if sendErr != nil {
			t.Fatalf("Send: %v", sendErr)
		}
		if len(invalid) != 0 {
			t.Fatalf("invalid = %v, want none (a transport failure is not an invalid token)", invalid)
		}
	})

	if len(spans) != 1 {
		t.Fatalf("expected 1 delivery span, got %d", len(spans))
	}
	span := spans[0]
	if span.Name() != "APNs delivery" {
		t.Errorf("span name = %q, want %q", span.Name(), "APNs delivery")
	}
	if span.Status().Code != codes.Error {
		t.Errorf("span status = %v, want Error", span.Status().Code)
	}

	if len(metricsRec.platforms) != 1 || metricsRec.platforms[0] != "apns" {
		t.Errorf("PushDeliveryFailed calls = %v, want [apns]", metricsRec.platforms)
	}
}

// TestClient_Send_RejectedTokenIsNotADeliveryFailure pins the churn-vs-failure
// distinction: a permanently invalid token (410 Unregistered) is expected
// churn the caller prunes, not a delivery failure — it must not mark the
// "APNs delivery" span as Error nor increment towncrier.push.delivery_failed.
func TestClient_Send_RejectedTokenIsNotADeliveryFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusGone)
		_, _ = w.Write([]byte(`{"reason":"Unregistered"}`))
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	metricsRec := &fakePushMetrics{}
	client.WithMetrics(metricsRec)

	spans := recordDeliverySpans(t, func() {
		invalid, sendErr := client.Send(context.Background(), []string{"deadtoken"}, json.RawMessage(`{}`))
		if sendErr != nil {
			t.Fatalf("Send: %v", sendErr)
		}
		if len(invalid) != 1 || invalid[0] != "deadtoken" {
			t.Fatalf("invalid = %v, want [deadtoken]", invalid)
		}
	})

	if len(spans) != 1 {
		t.Fatalf("expected 1 delivery span, got %d", len(spans))
	}
	if spans[0].Status().Code == codes.Error {
		t.Error("span status = Error, want unset for a routine token rejection")
	}
	if len(metricsRec.platforms) != 0 {
		t.Errorf("PushDeliveryFailed calls = %v, want none", metricsRec.platforms)
	}
}
