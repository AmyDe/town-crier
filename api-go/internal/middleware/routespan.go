package middleware

import (
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// routeMatcher is the subset of *http.ServeMux RouteSpan needs.
type routeMatcher interface {
	http.Handler
	Handler(r *http.Request) (h http.Handler, pattern string)
}

// statusWriter records the response status for the span, defaulting to 200 like
// net/http.
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

// RouteSpan names the inbound request span after the matched ServeMux route
// (e.g. "GET /v1/me") and records the HTTP status code on the span, which Azure
// Monitor maps to AppRequests.Name and ResultCode.
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
			// 4xx/5xx alerts keyed on ResultCode never fire. Do not "clean up"
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
