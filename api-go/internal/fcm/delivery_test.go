package fcm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
// the mirror helper in internal/apns/delivery_test.go).
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
// genuine per-device delivery failure (here, the OAuth token endpoint is
// unreachable, so sendOne never even gets to send the FCM request) must
// produce an "FCM delivery" span with Error status plus a
// towncrier.push.delivery_failed(platform=fcm) count — not just the
// "fcm send failed" slog line, which is all today's code emits.
func TestClient_Send_RecordsDeliveryFailureSpanAndMetric(t *testing.T) {
	fcmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer fcmSrv.Close()
	tokenSrv, _ := tokenServer(t)
	tokenSrv.Close() // force the OAuth token exchange to fail immediately

	client := newTestClient(t, fcmSrv, tokenSrv)
	metricsRec := &fakePushMetrics{}
	client.WithMetrics(metricsRec)

	spans := recordDeliverySpans(t, func() {
		invalid, err := client.Send(context.Background(), []string{"tok"}, json.RawMessage(`{"data":{}}`))
		if err != nil {
			t.Fatalf("Send: %v", err)
		}
		if len(invalid) != 0 {
			t.Fatalf("invalid = %v, want none (a transport failure is not an invalid token)", invalid)
		}
	})

	if len(spans) != 1 {
		t.Fatalf("expected 1 delivery span, got %d", len(spans))
	}
	span := spans[0]
	if span.Name() != "FCM delivery" {
		t.Errorf("span name = %q, want %q", span.Name(), "FCM delivery")
	}
	if span.Status().Code != codes.Error {
		t.Errorf("span status = %v, want Error", span.Status().Code)
	}

	if len(metricsRec.platforms) != 1 || metricsRec.platforms[0] != "fcm" {
		t.Errorf("PushDeliveryFailed calls = %v, want [fcm]", metricsRec.platforms)
	}
}

// TestClient_Send_RejectedTokenIsNotADeliveryFailure pins the churn-vs-failure
// distinction: a permanently invalid token (UNREGISTERED) is expected churn
// the caller prunes, not a delivery failure — it must not mark the "FCM
// delivery" span as Error nor increment towncrier.push.delivery_failed.
func TestClient_Send_RejectedTokenIsNotADeliveryFailure(t *testing.T) {
	fcmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":404,"status":"NOT_FOUND","message":"Requested entity was not found.","details":[{"@type":"type.googleapis.com/google.firebase.fcm.v1.FcmError","errorCode":"UNREGISTERED"}]}}`))
	}))
	defer fcmSrv.Close()
	tokenSrv, _ := tokenServer(t)

	client := newTestClient(t, fcmSrv, tokenSrv)
	metricsRec := &fakePushMetrics{}
	client.WithMetrics(metricsRec)

	spans := recordDeliverySpans(t, func() {
		invalid, err := client.Send(context.Background(), []string{"deadtoken"}, json.RawMessage(`{"data":{}}`))
		if err != nil {
			t.Fatalf("Send: %v", err)
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
