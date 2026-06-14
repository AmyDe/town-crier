package platform

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// installSpanRecorder swaps in an SDK TracerProvider backed by an in-memory span
// recorder for the duration of a test, restoring the previous global provider on
// cleanup (mirrors restoreOTelGlobals in telemetry_test.go).
func installSpanRecorder(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	prev := otel.GetTracerProvider()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(prev)
	})
	return rec
}

func attrLookup(attrs []attribute.KeyValue, key string) (attribute.Value, bool) {
	for _, a := range attrs {
		if string(a.Key) == key {
			return a.Value, true
		}
	}
	return attribute.Value{}, false
}

// TestTraceCosmosOp_RecordsClientSpan asserts the span helper produces a client
// span named "Cosmos <Op> <container>" carrying the OTel DB semantic-convention
// attributes App Insights needs to render the call as a dependency.
func TestTraceCosmosOp_RecordsClientSpan(t *testing.T) {
	rec := installSpanRecorder(t)

	c := &CosmosContainer{name: "Users", accountHost: "acct.documents.azure.com"}

	called := false
	err := traceCosmosOp(context.Background(), c, "ReadItem", func(context.Context) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("traceCosmosOp returned error: %v", err)
	}
	if !called {
		t.Fatal("operation func was not invoked")
	}

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	span := spans[0]

	if span.Name() != "Cosmos ReadItem Users" {
		t.Errorf("span name: got %q, want %q", span.Name(), "Cosmos ReadItem Users")
	}
	if span.SpanKind() != oteltrace.SpanKindClient {
		t.Errorf("span kind: got %v, want client", span.SpanKind())
	}

	attrs := span.Attributes()
	wantAttrs := map[string]string{
		"db.system":             "cosmosdb",
		"db.operation":          "ReadItem",
		"db.operation.name":     "ReadItem",
		"db.cosmosdb.container": "Users",
		"server.address":        "acct.documents.azure.com",
	}
	for key, want := range wantAttrs {
		got, ok := attrLookup(attrs, key)
		if !ok {
			t.Errorf("missing attribute %q", key)
			continue
		}
		if got.AsString() != want {
			t.Errorf("attribute %q: got %q, want %q", key, got.AsString(), want)
		}
	}

	if span.Status().Code != codes.Unset && span.Status().Code != codes.Ok {
		t.Errorf("status on success: got %v, want unset/ok", span.Status().Code)
	}
}

// TestTraceCosmosOp_RecordsError asserts a failing operation records the error
// on the span and sets Error status, so the failed dependency is queryable.
func TestTraceCosmosOp_RecordsError(t *testing.T) {
	rec := installSpanRecorder(t)

	c := &CosmosContainer{name: "NotificationState", accountHost: "acct.documents.azure.com"}
	sentinel := errors.New("cosmos timeout")

	err := traceCosmosOp(context.Background(), c, "QueryItems", func(context.Context) error {
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("traceCosmosOp must propagate the operation error: got %v", err)
	}

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	span := spans[0]

	if span.Status().Code != codes.Error {
		t.Errorf("status: got %v, want error", span.Status().Code)
	}
	if span.Status().Description != sentinel.Error() {
		t.Errorf("status description: got %q, want %q", span.Status().Description, sentinel.Error())
	}

	var foundException bool
	for _, ev := range span.Events() {
		if ev.Name == "exception" {
			foundException = true
		}
	}
	if !foundException {
		t.Error("expected an exception event from span.RecordError")
	}
}

// TestTraceCosmosOp_LinksChildSpan asserts the Cosmos span is a child of the
// incoming request span: started from the request ctx and propagated through to
// the SDK call so the trace is linked.
func TestTraceCosmosOp_LinksChildSpan(t *testing.T) {
	installSpanRecorder(t)

	c := &CosmosContainer{name: "Users", accountHost: "acct.documents.azure.com"}

	tracer := otel.Tracer("test")
	parentCtx, parent := tracer.Start(context.Background(), "request")

	var childTraceID oteltrace.TraceID
	err := traceCosmosOp(parentCtx, c, "UpsertItem", func(ctx context.Context) error {
		childTraceID = oteltrace.SpanContextFromContext(ctx).TraceID()
		return nil
	})
	if err != nil {
		t.Fatalf("traceCosmosOp: %v", err)
	}
	parent.End()

	if childTraceID != parent.SpanContext().TraceID() {
		t.Errorf("child span trace id %s != parent trace id %s; spans are not linked",
			childTraceID, parent.SpanContext().TraceID())
	}
}

// TestParseAccountHost extracts the account host from a Cosmos endpoint URL so
// the dependency Target is populated.
func TestParseAccountHost(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		endpoint string
		want     string
	}{
		{"https endpoint", "https://acct.documents.azure.com:443/", "acct.documents.azure.com"},
		{"no port", "https://acct.documents.azure.com/", "acct.documents.azure.com"},
		{"empty", "", ""},
		{"malformed falls back to raw", "://bad", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := parseAccountHost(tc.endpoint); got != tc.want {
				t.Errorf("parseAccountHost(%q) = %q, want %q", tc.endpoint, got, tc.want)
			}
		})
	}
}
