package platform

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// SpanNameFormatter names the client span for an outbound request. otelhttp's
// default formatter uses the HTTP method ("GET"), which makes AppDependencies
// rows indistinguishable across upstreams. Each outbound client supplies a
// stable, low-cardinality formatter (e.g. "PlanIt search", "Auth0 token") so the
// dependency Name plus the server.address Target identify the call. Formatters
// MUST NOT embed per-request identifiers (application IDs, device tokens) — that
// would explode span-name cardinality.
type SpanNameFormatter = func(operation string, r *http.Request) string

// WrapHTTPClient returns a copy of base whose transport is wrapped with
// otelhttp.NewTransport, so every request it makes emits an OTel client span
// (Type=HTTP in AppDependencies) and http.client.request.duration metrics
// through the global tracer/meter providers (installed by SetupTelemetry).
//
// The existing Timeout is preserved. When base has no explicit Transport,
// otelhttp wraps http.DefaultTransport (its documented fallback). The supplied
// formatter names the span; callers may pass extra otelhttp.Options (e.g.
// WithTracerProvider in hermetic tests).
//
// When telemetry is disabled (no OTLP endpoint) the global no-op providers make
// the wrapped transport produce no-op spans at negligible cost, so call sites
// wrap unconditionally.
func WrapHTTPClient(base *http.Client, formatter SpanNameFormatter, opts ...otelhttp.Option) *http.Client {
	wrapped := *base
	allOpts := append([]otelhttp.Option{otelhttp.WithSpanNameFormatter(formatter)}, opts...)
	wrapped.Transport = otelhttp.NewTransport(base.Transport, allOpts...)
	return &wrapped
}
