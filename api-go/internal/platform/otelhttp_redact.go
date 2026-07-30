package platform

import (
	"net/http"

	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	"go.opentelemetry.io/otel/trace"
)

// URLRedactor rewrites a request's URL for telemetry only — the real request
// sent over the wire is never touched. Implementations should return
// r.URL.String() unchanged unless the URL carries something sensitive.
type URLRedactor func(r *http.Request) string

// RedactingTransport wraps Next and, if the request carries a recording span
// (placed there by an outer otelhttp.Transport via WrapHTTPClient),
// overwrites that span's url.full attribute with Redact's result before the
// request reaches Next. otelhttp records the real URL into url.full when it
// starts the span, before calling the wrapped RoundTripper — there is no
// otelhttp Option to intercept that. The OTel SDK keeps the LAST value
// written for a duplicate attribute key (see recordingSpan.Attributes'
// dedupe-on-read in go.opentelemetry.io/otel/sdk/trace), so calling
// SetAttributes again here overwrites the exported value while leaving the
// outbound request and its URL completely unmodified.
//
// RedactingTransport is generic — it is not APNs-specific — so any outbound
// client whose URL embeds a per-user secret (a device token, a signed query
// parameter) can wrap its base transport with one before handing it to
// WrapHTTPClient.
type RedactingTransport struct {
	// Next performs the actual network round trip. Required.
	Next http.RoundTripper
	// Redact returns the telemetry-safe URL string for r. Required.
	Redact URLRedactor
}

// RoundTrip overwrites the current span's url.full attribute (if any) with
// Redact(r), then delegates to Next unchanged.
func (t *RedactingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if span := trace.SpanFromContext(r.Context()); span.IsRecording() {
		span.SetAttributes(semconv.URLFull(t.Redact(r)))
	}
	return t.Next.RoundTrip(r)
}
