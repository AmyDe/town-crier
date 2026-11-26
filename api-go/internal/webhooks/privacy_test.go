package webhooks

import (
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

const privateURL = "https://secret-host.example.com/services/SECRET-TOKEN?q=SECRET-QUERY"

// Drives the real delivery client, so the spans are the ones production emits.
func TestDrainAndTestRoute_LogsAndSpansNeverCarryURLOrHost(t *testing.T) {
	t.Parallel()
	respond := func(status int) roundTripFunc {
		return func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(privateURL))}, nil
		}
	}
	// A transport error quotes the host, as net and crypto/tls errors do.
	broken := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return nil, &net.OpError{Op: "dial", Err: errors.New("lookup " + r.URL.Host + " for " + r.URL.String())}
	})
	for _, tc := range []struct {
		name        string
		transport   roundTripFunc
		failuresYet int
		disabled    int
	}{
		{"success", respond(200), 0, 0},
		{"rejected", respond(500), 0, 0},
		{"transport error", broken, 0, 0},
		{"gone", respond(410), 0, 1},
		{"switched off after repeated failures", broken, MaxConsecutiveFailures - 1, 1},
	} {
		for _, format := range []Format{FormatJSON, FormatSlack} {
			t.Run(tc.name+" "+string(format), func(t *testing.T) {
				t.Parallel()
				rec := tracetest.NewSpanRecorder()
				tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
				client := newClient(tc.transport, func() time.Time { return drainNow }, otelhttp.WithTracerProvider(tp))
				ctx, root := tp.Tracer("test").Start(t.Context(), "root")

				drainFx := newDrainFixture()
				e := drainFx.store.addEndpoint("e1", format)
				e.URL, e.ConsecutiveFailures = privateURL, tc.failuresYet
				drainFx.store.addRows(t, "e1", "d1", "d2")
				// A failing email and store make the error paths log too.
				drainFx.email.err = errors.New("acs rejected")
				logger := slog.New(slog.NewJSONHandler(drainFx.logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
				got := NewDrainer(drainFx.store, client, drainFx.profiles, drainFx.email, logger,
					func() time.Time { return drainNow }).Drain(ctx)

				handlerFx := newHandlerFixture()
				stored := handlerFx.withEndpoint()
				stored.URL, stored.Format = privateURL, format
				response := handlerFx.serveWith(ctx, client, "POST", "/v1/me/webhook/test", "")
				root.End()

				if got.Disabled != tc.disabled || response.Code != http.StatusOK {
					t.Fatalf("drain = %+v, test route = %d", got, response.Code)
				}
				spans := rec.Ended()
				if len(spans) < 3 {
					t.Fatalf("spans = %d, want the deliveries, the test and the root", len(spans))
				}
				output := map[string]string{
					"spans":               spanText(spans),
					"drain logs":          drainFx.logs.String(),
					"handler logs":        handlerFx.logs.String(),
					"test route response": response.Body.String(),
				}
				if !strings.Contains(output["drain logs"], `"msg":"webhook.drain"`) {
					t.Fatalf("the logs were not captured: %s", output["drain logs"])
				}
				for where, text := range output {
					for _, leak := range []string{"secret-host", "example.com", "SECRET", "/services"} {
						if strings.Contains(text, leak) {
							t.Errorf("%s carry %q:\n%s", where, leak, text)
						}
					}
				}
			})
		}
	}
}
