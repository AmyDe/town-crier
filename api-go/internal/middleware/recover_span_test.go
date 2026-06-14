package middleware

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// spanStarter mimics otelhttp: it begins a server span on the request ctx so
// the Recover middleware (further in) can record the panic on the active span.
func spanStarter(next http.Handler) http.Handler {
	tracer := otel.Tracer("test")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(r.Context(), "request", oteltrace.WithSpanKind(oteltrace.SpanKindServer))
		defer span.End()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// TestRecover_PanicRecordsSpanException pins tc-8x8g: a panic in a handler must
// record an exception event on the active request span and set Error status, so
// it surfaces in App Insights AppExceptions and the request shows as failed.
func TestRecover_PanicRecordsSpanException(t *testing.T) {
	prev := otel.GetTracerProvider()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(prev)
	})

	logger := slog.New(slog.DiscardHandler)
	chain := spanStarter(ErrorBody(logger)(Recover(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom: handler exploded")
	}))))
	srv := httptest.NewServer(chain)
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", resp.StatusCode)
	}

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	span := spans[0]

	if span.Status().Code != codes.Error {
		t.Errorf("span status: got %v, want error", span.Status().Code)
	}
	if span.Status().Description != "boom: handler exploded" {
		t.Errorf("span status description: got %q, want %q", span.Status().Description, "boom: handler exploded")
	}

	var foundException bool
	for _, ev := range span.Events() {
		if ev.Name == "exception" {
			foundException = true
		}
	}
	if !foundException {
		t.Error("expected an exception event recorded on the request span")
	}
}
