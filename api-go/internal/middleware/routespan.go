package middleware

import (
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// routeMatcher is the subset of *http.ServeMux RouteSpan needs to resolve the
// pattern a request matches, mirroring auth.RequireAuth's matcher. Declared
// here, consumer-side, so the middleware depends only on the two methods it
// uses and *http.ServeMux satisfies it implicitly.
type routeMatcher interface {
	http.Handler
	Handler(r *http.Request) (h http.Handler, pattern string)
}

// statusWriter captures the response status code so RouteSpan can record it on
// the request span. It defaults to 200, matching net/http's behaviour when a
// handler writes a body without an explicit WriteHeader.
type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (s *statusWriter) WriteHeader(status int) {
	if !s.wroteHeader {
		s.status = status
		s.wroteHeader = true
	}
	s.ResponseWriter.WriteHeader(status)
}

func (s *statusWriter) Write(p []byte) (int, error) {
	if !s.wroteHeader {
		s.status = http.StatusOK
		s.wroteHeader = true
	}
	return s.ResponseWriter.Write(p)
}

// Unwrap lets http.ResponseController reach the underlying writer (Flush etc.).
func (s *statusWriter) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// RouteSpan restores the request-telemetry parity the .NET API had (tc-r8eo):
// it names the inbound request span after the matched ServeMux route (e.g.
// "GET /v1/me") and records the real HTTP status code on the span. Azure Monitor
// maps span Name -> AppRequests.Name and http.response.status_code ->
// ResultCode, so without this every Go request row showed only the bare verb and
// a span-status ResultCode (0/2), leaving status-keyed dashboards and 4xx/5xx
// alerts blind.
//
// It resolves the pattern up front via the matcher (the same deterministic
// lookup auth.RequireAuth uses) rather than reading r.Pattern, because the
// request that the inner mux mutates is a context-copy that never propagates
// back here. An unmatched request leaves the default span name and sets no
// http.route, but still records its status code. Place it within the otelhttp
// span — wrapping the rest of the chain — so the active span is the request span.
func RouteSpan(matcher routeMatcher) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, pattern := matcher.Handler(r)

			span := trace.SpanFromContext(r.Context())
			if pattern != "" {
				span.SetName(pattern)
				span.SetAttributes(semconv.HTTPRoute(pattern))
			}

			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)

			// Set both the stable semconv key and the legacy http.status_code (pre-1.21
		// semconv). Azure Monitor maps AppRequests.ResultCode from the LEGACY
		// http.status_code, NOT the stable http.response.status_code, so without the
		// legacy attribute ResultCode shows only the bare span StatusCode (0/2) and
		// 4xx/5xx alerts keyed on ResultCode never fire (tc-oml9). Do not "clean up"
		// this apparent duplication — both keys are load-bearing.
		span.SetAttributes(
			semconv.HTTPResponseStatusCode(sw.status),
			attribute.Int("http.status_code", sw.status),
		)
			// Mirror semconv server semantics: a 5xx is a request error. ErrorBody
			// also flags 5xx, but it does not run for the unmatched-route 404/anonymous
			// paths that bypass it, so set it here too. SetStatus is idempotent and the
			// last Error wins, so re-flagging a panic-marked span is harmless.
			if sw.status >= http.StatusInternalServerError {
				span.SetStatus(codes.Error, http.StatusText(sw.status))
			}
		})
	}
}
