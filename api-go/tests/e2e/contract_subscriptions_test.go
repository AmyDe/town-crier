//go:build e2e

package e2e

import (
	"net/http"
	"testing"
)

// Contract scenarios for the iteration-9 subscriptions surface. Only the
// deterministic, secret-free paths are diffed here — none needs a real
// Apple-signed JWS:
//
//   - The webhook (anonymous) rejects a malformed body / empty payload with the
//     malformed_request envelope and a structurally-invalid JWS with the
//     invalid_notification 401 — all before any Cosmos access.
//   - Verify (authed) short-circuits a malformed body before any profile load,
//     and fails a structurally-invalid JWS identically on both APIs.
//
// The happy path — a real verified transaction activating a tier — cannot be
// diffed without an Apple-signed payload (App Store Connect StoreKit signing),
// so it is deferred to tc-dpfn alongside the App Store Server Notification
// replay. The two APIs share one Cosmos account, so the verify error scenarios
// return the same result whatever the shared user's profile state.

func TestContract_AppStoreWebhook(t *testing.T) {
	dotnetURL := baseURL(t, "DOTNET_BASE_URL")
	goURL := baseURL(t, "GO_BASE_URL")
	client := &http.Client{Timeout: requestTimeout}

	// The webhook is anonymous, so no token is supplied. The signed JWS is the
	// authentication, and these payloads fail that check identically.
	t.Run("malformed body", func(t *testing.T) {
		diffWatchZone(t, client, dotnetURL, goURL, http.MethodPost, "/v1/webhooks/appstore", "", "{not json")
	})
	t.Run("empty payload", func(t *testing.T) {
		diffWatchZone(t, client, dotnetURL, goURL, http.MethodPost, "/v1/webhooks/appstore", "", `{"signedPayload":""}`)
	})
	t.Run("invalid jws", func(t *testing.T) {
		diffWatchZone(t, client, dotnetURL, goURL, http.MethodPost, "/v1/webhooks/appstore", "", `{"signedPayload":"not-a-jws"}`)
	})
}

func TestContract_VerifySubscription_Errors(t *testing.T) {
	dotnetURL := baseURL(t, "DOTNET_BASE_URL")
	goURL := baseURL(t, "GO_BASE_URL")
	token := integrationToken(t)
	client := &http.Client{Timeout: requestTimeout}

	// Setup, not a diff: ensure the shared profile exists so the JWS path is
	// reached (verify loads the caller's profile first; a missing one is 404).
	if setup := watchZoneRequest(t, client, dotnetURL, http.MethodPost, "/v1/me", token, ""); setup.status >= 500 {
		t.Fatalf("setup POST /v1/me on .NET: %d %s", setup.status, setup.body)
	}

	// A malformed body short-circuits before any profile load on both APIs.
	t.Run("malformed body", func(t *testing.T) {
		diffWatchZone(t, client, dotnetURL, goURL, http.MethodPost, "/v1/subscriptions/verify", token, "{not json")
	})
	// A structurally-invalid JWS fails verification with the invalid_transaction
	// 401 envelope on both APIs.
	t.Run("invalid jws", func(t *testing.T) {
		diffWatchZone(t, client, dotnetURL, goURL, http.MethodPost, "/v1/subscriptions/verify", token, `{"signedTransaction":"not-a-jws"}`)
	})
}
