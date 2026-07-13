package auth

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// spanStarter mimics otelhttp (the outermost wrapper in production, see
// cmd/api/wiring.go): it begins a server span on the request context before
// RequireAuth runs, so the middleware has a live span to stamp attributes on.
func spanStarter(next http.Handler) http.Handler {
	tracer := otel.Tracer("test")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(r.Context(), "request", oteltrace.WithSpanKind(oteltrace.SpanKindServer))
		defer span.End()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// serveAndRecordSpan runs a request through spanStarter -> h and returns the
// single recorded request span so tests can assert its attributes.
func serveAndRecordSpan(t *testing.T, h http.Handler, path string, configure func(*http.Request)) sdktrace.ReadOnlySpan {
	t.Helper()

	prev := otel.GetTracerProvider()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(prev)
	})

	srv := httptest.NewServer(spanStarter(h))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+path, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if configure != nil {
		configure(req)
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

// spanAttr looks up a single attribute by key on the recorded span.
func spanAttr(span sdktrace.ReadOnlySpan, key attribute.Key) (attribute.Value, bool) {
	for _, a := range span.Attributes() {
		if a.Key == key {
			return a.Value, true
		}
	}
	return attribute.Value{}, false
}

// TestRequireAuth_StampsEnduserIDOnSuccessfulAuth pins tc-p7az9: a
// successfully authenticated request must carry the Auth0 subject as
// enduser.id on the live request span, so the Daily Active Users dashboard
// tile (tc-gha6l) can chart true active users from Azure Monitor telemetry.
func TestRequireAuth_StampsEnduserIDOnSuccessfulAuth(t *testing.T) {
	t.Parallel()

	mux, anonymous := muxWith(t)
	v := &fakeValidator{subjectByToken: map[string]string{"good": "auth0|abc123"}}
	h := RequireAuth(v, mux, anonymous)

	span := serveAndRecordSpan(t, h, "/api/me", func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer good")
	})

	got, ok := spanAttr(span, semconv.EnduserIDKey)
	if !ok {
		t.Fatalf("enduser.id attribute missing on authenticated request span")
	}
	if got.AsString() != "auth0|abc123" {
		t.Errorf("enduser.id = %q, want %q", got.AsString(), "auth0|abc123")
	}
}

// TestRequireAuth_DoesNotStampEnduserIDOnAnonymousRoute confirms anonymous
// requests (no token presented, no validation attempted) get no enduser.id
// attribute — GDPR minimisation: only a successfully authenticated subject is
// ever stamped.
func TestRequireAuth_DoesNotStampEnduserIDOnAnonymousRoute(t *testing.T) {
	t.Parallel()

	mux, anonymous := muxWith(t)
	v := &fakeValidator{}
	h := RequireAuth(v, mux, anonymous)

	span := serveAndRecordSpan(t, h, "/v1/health", nil)

	if _, ok := spanAttr(span, semconv.EnduserIDKey); ok {
		t.Errorf("enduser.id attribute present on anonymous route, want absent")
	}
}

// TestRequireAuth_DoesNotStampEnduserIDOnFailedAuth confirms a request that
// fails authentication (missing or invalid token) gets no enduser.id
// attribute — only successful token validation stamps the span.
func TestRequireAuth_DoesNotStampEnduserIDOnFailedAuth(t *testing.T) {
	t.Parallel()

	mux, anonymous := muxWith(t)
	v := &fakeValidator{subjectByToken: map[string]string{"good": "auth0|abc123"}}
	h := RequireAuth(v, mux, anonymous)

	span := serveAndRecordSpan(t, h, "/api/me", func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer bad")
	})

	if _, ok := spanAttr(span, semconv.EnduserIDKey); ok {
		t.Errorf("enduser.id attribute present on failed auth, want absent")
	}
}
