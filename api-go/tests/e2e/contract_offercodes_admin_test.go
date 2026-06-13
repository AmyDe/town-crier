//go:build e2e

package e2e

import (
	"net/http"
	"testing"
)

// Contract scenarios for the iteration-8 offer-code and admin surface. Only the
// deterministic, side-effect-free paths are diffed here:
//
//   - Redeem's invalid-format (400) and unknown-code (404) errors short-circuit
//     before any Cosmos write and before the profile is loaded, so they are
//     identical on both APIs regardless of the shared user's subscription state.
//   - The admin surface's no-key 401 needs no shared secret.
//
// The stateful / secret-dependent diffs — redeem happy path (needs a seeded
// code), already-redeemed / already-subscribed (need specific state), and the
// admin-authenticated grant/list/generate bodies (need a shared ADMIN_API_KEY on
// the Go app, plus opaque continuation tokens that differ across SDKs) — are
// deferred to tc-52t6.

func TestContract_OfferCodeRedeem_Errors(t *testing.T) {
	dotnetURL := baseURL(t, "DOTNET_BASE_URL")
	goURL := baseURL(t, "GO_BASE_URL")
	token := integrationToken(t)
	client := &http.Client{Timeout: requestTimeout}

	// A malformed code is rejected at the boundary with the invalid_code_format
	// envelope, before any lookup or write.
	t.Run("invalid format", func(t *testing.T) {
		diffWatchZone(t, client, dotnetURL, goURL, http.MethodPost, "/v1/offer-codes/redeem", token, `{"code":"ABCD"}`)
	})

	// A well-formed code that does not exist is a 404 invalid_code. ZZZZZZZZZZZZ
	// is a valid canonical code that is astronomically unlikely to have been
	// minted (60-bit random space), so it is absent on both APIs' shared Cosmos.
	t.Run("unknown code", func(t *testing.T) {
		diffWatchZone(t, client, dotnetURL, goURL, http.MethodPost, "/v1/offer-codes/redeem", token, `{"code":"ZZZZZZZZZZZZ"}`)
	})
}

func TestContract_AdminRequiresKey(t *testing.T) {
	dotnetURL := baseURL(t, "DOTNET_BASE_URL")
	goURL := baseURL(t, "GO_BASE_URL")
	client := &http.Client{Timeout: requestTimeout}

	// The admin endpoints are anonymous to Auth0 but gated by X-Admin-Key. With no
	// key, GET /v1/admin/users returns a bodyless 401 (the PascalCase envelope
	// backfilled) and — unlike the Auth0 fallback-deny — no WWW-Authenticate
	// header. (Only the GET route is exercised here; the PUT/POST admin routes
	// would method-mismatch under diffChallenge's GET.)
	diffChallenge(t, client, dotnetURL, goURL, "/v1/admin/users")
}
