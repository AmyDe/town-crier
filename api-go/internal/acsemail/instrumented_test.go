package acsemail

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// fakeSender is a hand-written double for the EmailSender InstrumentedSender
// wraps.
type fakeSender struct {
	calls int
	sent  []Message
	err   error
}

func (f *fakeSender) Send(_ context.Context, msg Message) error {
	f.calls++
	f.sent = append(f.sent, msg)
	return f.err
}

// recordEmailSpan swaps in an in-memory SDK TracerProvider for the duration of
// run, restoring the previous global provider on cleanup, and returns the
// single recorded span. Deliberately not t.Parallel(): mutating the global
// TracerProvider is safe only while no sibling test's body is concurrently
// executing (matches the convention already used in
// internal/middleware/*_span_test.go and internal/worker/dispatch_test.go).
func recordEmailSpan(t *testing.T, run func()) sdktrace.ReadOnlySpan {
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

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 recorded span, got %d", len(spans))
	}
	return spans[0]
}

func attrString(span sdktrace.ReadOnlySpan, key string) (string, bool) {
	for _, kv := range span.Attributes() {
		if string(kv.Key) == key {
			return kv.Value.AsString(), true
		}
	}
	return "", false
}

// TestInstrumentedSender_EmitsEmailSendSpanWithKind pins tc-3jx8d: exactly one
// "Email send" wrapper span per Send call, tagged with the caller's kind, so
// per-email delivery is visible as a dependency distinct from the low-level
// "ACS email send" HTTP client spans (unaffected by this wrapper).
func TestInstrumentedSender_EmitsEmailSendSpanWithKind(t *testing.T) {
	next := &fakeSender{}
	s := NewInstrumentedSender(next)
	msg := testMessage()

	span := recordEmailSpan(t, func() {
		if err := s.Send(context.Background(), "digest-weekly", msg); err != nil {
			t.Fatalf("Send: %v", err)
		}
	})

	if span.Name() != "Email send" {
		t.Errorf("span name: got %q, want %q", span.Name(), "Email send")
	}
	if got, ok := attrString(span, "email.kind"); !ok || got != "digest-weekly" {
		t.Errorf("email.kind: got %q (present=%v), want %q", got, ok, "digest-weekly")
	}
	if span.Status().Code == codes.Error {
		t.Error("span status: got Error, want unset for a successful send")
	}
	if next.calls != 1 || len(next.sent) != 1 || next.sent[0] != msg {
		t.Errorf("wrapped sender: calls=%d sent=%v, want exactly 1 call with the original message", next.calls, next.sent)
	}
}

// TestInstrumentedSender_RecordsErrorAndStatusOnFailure asserts a failed send
// is recorded on the wrapper span (RecordError + Error status) so the
// dependency's success flag reflects the outcome, and that the underlying
// error still propagates to the caller.
func TestInstrumentedSender_RecordsErrorAndStatusOnFailure(t *testing.T) {
	sendErr := errors.New("acs rejected")
	next := &fakeSender{err: sendErr}
	s := NewInstrumentedSender(next)

	var gotErr error
	span := recordEmailSpan(t, func() {
		gotErr = s.Send(context.Background(), "digest-hourly", testMessage())
	})

	if !errors.Is(gotErr, sendErr) {
		t.Errorf("Send error: got %v, want %v", gotErr, sendErr)
	}
	if span.Status().Code != codes.Error {
		t.Errorf("span status: got %v, want Error", span.Status().Code)
	}
	if span.Status().Description != sendErr.Error() {
		t.Errorf("span status description: got %q, want %q", span.Status().Description, sendErr.Error())
	}
	var foundException bool
	for _, ev := range span.Events() {
		if ev.Name == "exception" {
			foundException = true
		}
	}
	if !foundException {
		t.Error("expected an exception event recorded on the span")
	}
	if got, ok := attrString(span, "email.kind"); !ok || got != "digest-hourly" {
		t.Errorf("email.kind: got %q (present=%v), want %q", got, ok, "digest-hourly")
	}
}
