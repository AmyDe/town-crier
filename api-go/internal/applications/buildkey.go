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
// Exported so a caller outside this package can recognise build-key-
// authenticated traffic without duplicating the header name or the
// constant-time compare — specifically the anonymous-rate-limit exemption
// predicate wired in cmd/api/wiring.go (GH#872 collateral, tc-zod82): the
// build-key SEO endpoints authenticate inside the handler rather than via
// Auth0, so without this exemption their traffic is wrongly metered as
// anonymous.
func BuildKeyMatches(r *http.Request, expectedKey string) bool {
	if expectedKey == "" {
		return false
	}
	provided := []byte(r.Header.Get(buildKeyHeader))
	expected := []byte(expectedKey)
	return subtle.ConstantTimeCompare(provided, expected) == 1
}

// requireBuildKey wraps next with the build-key gate for the build-time SEO
// endpoint: a request lacking a matching X-Build-Key gets a bodyless 401 (the
// PascalCase envelope is backfilled by middleware.ErrorBody). An empty configured
// key rejects every request, so an unconfigured deployment cannot be reached. The
// compare is constant-time to avoid leaking the key by timing (BuildKeyMatches).
//
// This is a separate gate from admin.requireAdminKey by design (least privilege):
// the SEO endpoint reads only public planning data, so it must not share the
// high-blast-radius admin key.
func requireBuildKey(expectedKey string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !BuildKeyMatches(r, expectedKey) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}
