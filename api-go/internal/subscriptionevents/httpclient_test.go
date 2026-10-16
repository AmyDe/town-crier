package subscriptionevents

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestHTTPClient_SpanURLFullOmitsWebhookSecrets(t *testing.T) {
	t.Parallel()

	var gotURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURI = r.URL.RequestURI()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	client := newHTTPClient(http.DefaultTransport, otelhttp.WithTracerProvider(tp))

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/services/T1/B2/SECRET-TOKEN?x=y", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	if gotURI != "/services/T1/B2/SECRET-TOKEN?x=y" {
		t.Fatalf("server saw %q, want full path", gotURI)
	}
	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	var full string
	for _, kv := range spans[0].Attributes() {
		if kv.Key == attribute.Key("url.full") {
			full = kv.Value.AsString()
		}
	}
	if full == "" {
		t.Fatal("url.full not recorded")
	}
	for _, leak := range []string{"SECRET-TOKEN", "/services", "x=y", "?"} {
		if strings.Contains(full, leak) {
			t.Errorf("url.full %q leaks %q", full, leak)
		}
	}
}
