package applications

import (
	"crypto/subtle"
	"net/http"
)

// buildKeyHeader is the header carrying the dedicated site-build key.
const buildKeyHeader = "X-Build-Key"

// requireBuildKey wraps next with the build-key gate for the build-time SEO
// endpoint: a request lacking a matching X-Build-Key gets a bodyless 401 (the
// PascalCase envelope is backfilled by middleware.ErrorBody). An empty configured
// key rejects every request, so an unconfigured deployment cannot be reached. The
// compare is constant-time to avoid leaking the key by timing.
//
// This is a separate gate from admin.requireAdminKey by design (least privilege):
// the SEO endpoint reads only public planning data, so it must not share the
// high-blast-radius admin key.
func requireBuildKey(expectedKey string, next http.HandlerFunc) http.HandlerFunc {
	expected := []byte(expectedKey)
	return func(w http.ResponseWriter, r *http.Request) {
		provided := []byte(r.Header.Get(buildKeyHeader))
		if len(expected) == 0 || subtle.ConstantTimeCompare(provided, expected) != 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}
