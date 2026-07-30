package platform

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// TestRedactingTransport_OverwritesURLFullWithoutTouchingRequest asserts that
// wrapping the innermost transport with RedactingTransport rewrites the
// recorded span's url.full attribute using Redact's return value, while the
// request that actually reaches the server (via Next) still carries the
// original, unredacted URL.
func TestRedactingTransport_OverwritesURLFullWithoutTouchingRequest(t *testing.T) {
	t.Parallel()

	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	tp, rec := newRecorderProvider(t)

	base := &http.Client{
		Transport: &RedactingTransport{
			Next: http.DefaultTransport,
			Redact: func(r *http.Request) string {
				return "https://example.invalid/redacted"
			},
		},
	}
	wrapped := WrapHTTPClient(base, func(string, *http.Request) string { return "Test push" }, otelhttp.WithTracerProvider(tp))

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/3/device/super-secret-token", http.NoBody)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := wrapped.Do(req)
	if err != nil {
		t.Fatalf("wrapped.Do: %v", err)
	}
	_ = resp.Body.Close()

	// The real network request must be untouched by the redaction.
	if gotPath != "/3/device/super-secret-token" {
		t.Fatalf("server saw path %q, want unredacted /3/device/super-secret-token", gotPath)
	}

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 client span, got %d", len(spans))
	}
	span := spans[0]

	got, ok := attrLookupKV(span.Attributes(), "url.full")
	if !ok {
		t.Fatalf("missing url.full attribute; attrs=%v", span.Attributes())
	}
	if got.AsString() != "https://example.invalid/redacted" {
		t.Errorf("url.full: got %q, want redacted value", got.AsString())
	}
	if strings.Contains(got.AsString(), "super-secret-token") {
		t.Errorf("url.full leaked the raw token: %q", got.AsString())
	}
}
