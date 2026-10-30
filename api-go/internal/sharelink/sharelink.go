// Package sharelink reads the short random code that the iOS app adds to a
// shared link. The code measures clicks on a link: it is never tied to an
// account, an install or a signup.
package sharelink

import (
	"net/http"
	"regexp"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
)

const (
	queryParam    = "s"
	spanAttribute = "share.nonce"
)

var codePattern = regexp.MustCompile(`^[A-Za-z0-9]{6}$`)

// Record returns the validated share code from the "s" query parameter and sets
// it on the request span as share.nonce. It returns "" and sets nothing when the
// code is absent or invalid, or when the request carries an account.
//
// The Authorization check matters because anonymous routes never validate a
// token, so a signed-in client's request has no subject on these routes.
func Record(r *http.Request) string {
	if auth.Subject(r.Context()) != "" || r.Header.Get("Authorization") != "" {
		return ""
	}
	code := r.URL.Query().Get(queryParam)
	if !codePattern.MatchString(code) {
		return ""
	}
	trace.SpanFromContext(r.Context()).SetAttributes(attribute.String(spanAttribute, code))
	return code
}
