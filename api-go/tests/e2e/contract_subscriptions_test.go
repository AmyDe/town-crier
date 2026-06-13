//go:build e2e

package e2e

import (
	"net/http"
	"testing"
)

// Contract scenarios for the iteration-9 subscriptions surface. Only the
// malformed-body paths are diffed — they reach the same 400 malformed_request
// envelope on both APIs before any binding or JWS work.
//
// The JWS-failure paths (a structurally-invalid signedTransaction/signedPayload)
// CANNOT be diffed against .NET, because .NET's request DTOs are bound
// case-sensitively as PascalCase: the body is read via the source-generated
// JsonTypeInfo (AppJsonSerializerContext), which ignores ASP.NET's camelCase
// PropertyNamingPolicy. The real iOS app sends camelCase {"signedTransaction":...}
// and Apple's webhook sends camelCase {"signedPayload":...}, so neither binds on
// .NET -> the field is null -> .NET returns 400 malformed_request. In other
// words the .NET verify/webhook endpoints never bound a real request — the same
// class of end-to-end bug as the .NET product-ID typo (tc-7g3i.12). Go binds the
// camelCase body (encoding/json is case-insensitive) and is the first correct
// implementation, so its JWS-failure path (401) diverges from .NET's bugged 400
// by design. That path is unit-tested in internal/subscriptions; the happy path
// (a real Apple-signed payload) is deferred to tc-dpfn (blocks the it10 cutover).
//
// The two APIs share one Cosmos account, so the error scenarios below return the
// same result whatever the shared user's profile state.

func TestContract_AppStoreWebhook(t *testing.T) {
	dotnetURL := baseURL(t, "DOTNET_BASE_URL")
	goURL := baseURL(t, "GO_BASE_URL")
	client := &http.Client{Timeout: requestTimeout}

	// The webhook is anonymous (the signed JWS is its auth), so no token is sent.
	t.Run("malformed body", func(t *testing.T) {
		diffWatchZone(t, client, dotnetURL, goURL, http.MethodPost, "/v1/webhooks/appstore", "", "{not json")
	})
	// An empty/absent signedPayload is a malformed request on both — .NET because
	// PascalCase binding leaves it null, Go because the camelCase field is blank.
	t.Run("empty payload", func(t *testing.T) {
		diffWatchZone(t, client, dotnetURL, goURL, http.MethodPost, "/v1/webhooks/appstore", "", `{"signedPayload":""}`)
	})
}

func TestContract_VerifySubscription_MalformedBody(t *testing.T) {
	dotnetURL := baseURL(t, "DOTNET_BASE_URL")
	goURL := baseURL(t, "GO_BASE_URL")
	token := integrationToken(t)
	client := &http.Client{Timeout: requestTimeout}

	// A malformed body short-circuits to 400 malformed_request before any binding
	// or profile load on both APIs. (verify is authed, so a token is supplied.)
	diffWatchZone(t, client, dotnetURL, goURL, http.MethodPost, "/v1/subscriptions/verify", token, "{not json")
}
