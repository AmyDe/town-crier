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
)

// serveSpanChain runs a request through spanStarter -> ErrorBody -> handler and
// returns the single recorded request span so tests can assert its status.
func serveSpanChain(t *testing.T, next http.HandlerFunc) sdktrace.ReadOnlySpan {
	t.Helper()

	prev := otel.GetTracerProvider()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(prev)
	})

	logger := slog.New(slog.DiscardHandler)
	chain := spanStarter(ErrorBody(logger)(next))
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

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	return spans[0]
}

// TestErrorBody_500MarksRequestSpanFailed pins tc-8x8g task D: a handler that
// returns a bodyless 500 (e.g. the Cosmos-timeout notification-state path) must
// leave the request span marked Error so AppRequests shows it as failed and the
// 500 is queryable — without the handler having to RecordError itself.
func TestErrorBody_500MarksRequestSpanFailed(t *testing.T) {
	span := serveSpanChain(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	if span.Status().Code != codes.Error {
		t.Errorf("span status for 500: got %v, want error", span.Status().Code)
	}
}

// TestErrorBody_502MarksRequestSpanFailed covers a non-500 5xx written with a
// body, confirming the >= 500 threshold (not just == 500) flags the span.
func TestErrorBody_502MarksRequestSpanFailed(t *testing.T) {
	span := serveSpanChain(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("upstream down"))
	})
	if span.Status().Code != codes.Error {
		t.Errorf("span status for 502: got %v, want error", span.Status().Code)
	}
}

// TestErrorBody_4xxDoesNotMarkSpanFailed confirms client errors (4xx) do NOT
// flag the request span as failed — only server errors (>= 500) do.
func TestErrorBody_4xxDoesNotMarkSpanFailed(t *testing.T) {
	span := serveSpanChain(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	if span.Status().Code == codes.Error {
		t.Errorf("span status for 404: got error, want unset/ok (4xx is a client error)")
	}
}

// TestErrorBody_200DoesNotMarkSpanFailed confirms the happy path leaves the
// span unflagged.
func TestErrorBody_200DoesNotMarkSpanFailed(t *testing.T) {
	span := serveSpanChain(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	if span.Status().Code == codes.Error {
		t.Errorf("span status for 200: got error, want unset/ok")
	}
}
