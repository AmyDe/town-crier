package applications

import (
	"crypto/subtle"
	"net/http"
)

// buildKeyHeader is the header carrying the dedicated site-build key.
const buildKeyHeader = "X-Build-Key"

// BuildKeyMatches reports whether r carries an X-Build-Key header that
// constant-time-matches expectedKey. An empty expectedKey NEVER matches, even
// against an empty or absent header: crypto/subtle.ConstantTimeCompare
// returns 1 for two empty byte slices, so without this guard an unconfigured
// deployment (SITE_BUILD_KEY unset) would treat a keyless caller as a match
// instead of rejecting every request.
//
// Exported for the SEO handlers (internal/seopage) and the anonymous-rate-limit
// exemption in cmd/api/wiring.go: the build-key SEO endpoints authenticate inside
// the handler rather than via Auth0, so without it their traffic is metered as
// anonymous.
func BuildKeyMatches(r *http.Request, expectedKey string) bool {
	if expectedKey == "" {
		return false
	}
	provided := []byte(r.Header.Get(buildKeyHeader))
	expected := []byte(expectedKey)
	return subtle.ConstantTimeCompare(provided, expected) == 1
}
