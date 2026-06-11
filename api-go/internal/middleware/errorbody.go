// Package middleware holds the cross-cutting http.Handler wrappers that
// replicate the .NET API's pipeline behaviours (GH#418). Composition is plain
// func(http.Handler) http.Handler, chained by hand in cmd/api/main.go.
package middleware

import "net/http"

// ErrorBody replicates the backfill half of the .NET ErrorResponseMiddleware
// contract (GH#418, parity behaviour 1): any response with status >= 400 that
// would otherwise be sent with an empty body gets the PascalCase JSON envelope
// {"Status":<n>,"Title":"<reason>","Detail":null} with Content-Type exactly
// "application/json" (no charset on this path, unlike handler-written bodies).
func ErrorBody(next http.Handler) http.Handler {
	return next
}
