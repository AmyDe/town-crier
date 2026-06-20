// Package admin owns the admin surface: the X-Admin-Key gate and the
// PUT /v1/admin/subscriptions, GET /v1/admin/users, POST /v1/admin/offer-codes
// handlers. The admin routes are anonymous to Auth0 (absent from the
// fallback-deny set) and authenticated solely by the shared admin key.
package admin

import (
	"crypto/subtle"
	"net/http"
)

// adminKeyHeader is the header carrying the shared admin key.
const adminKeyHeader = "X-Admin-Key"

// requireAdminKey wraps next with the shared-key gate: a request lacking a
// matching X-Admin-Key gets a bodyless 401 (the PascalCase envelope is
// backfilled by middleware.ErrorBody). An empty configured key rejects every
// request, so an unconfigured deployment cannot be reached. The compare is
// constant-time to avoid leaking the key by timing.
func requireAdminKey(expectedKey string, next http.HandlerFunc) http.HandlerFunc {
	expected := []byte(expectedKey)
	return func(w http.ResponseWriter, r *http.Request) {
		provided := []byte(r.Header.Get(adminKeyHeader))
		if len(expected) == 0 || subtle.ConstantTimeCompare(provided, expected) != 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}
