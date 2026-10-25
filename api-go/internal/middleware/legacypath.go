package middleware

import (
	"net/http"
	"net/url"
	"strings"
)

// legacyTokenSegment is the path segment of the deprecated route that carries
// a device token in the segment after it.
const legacyTokenSegment = "/device-token/"

// TraceFilter is an otelhttp filter: it reports false for requests that must
// not produce a span because their path carries a device token. It matches the
// segment anywhere in the path, case-insensitively, because otelhttp sees the
// path before ServeMux canonicalises it.
func TraceFilter(r *http.Request) bool {
	if hasLegacySegment(r.URL.Path) || hasLegacySegment(r.URL.RawPath) {
		return false
	}
	if raw, err := url.PathUnescape(r.URL.RawPath); err == nil && hasLegacySegment(raw) {
		return false
	}
	return true
}

// RedactPath returns path with everything after the device-token segment
// replaced, for use in log output.
func RedactPath(path string) string {
	i := strings.Index(strings.ToLower(path), legacyTokenSegment)
	if i < 0 {
		return path
	}
	return path[:i+len(legacyTokenSegment)] + "{redacted}"
}

func hasLegacySegment(path string) bool {
	return strings.Contains(strings.ToLower(path), legacyTokenSegment)
}
