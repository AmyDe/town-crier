//go:build e2e

package e2e

import (
	"net/http"
	"testing"
)

// TestContract_DemoAccount diffs GET /v1/demo-account between the deployed .NET
// and Go APIs. The endpoint is anonymous (Apple's reviewer reaches it without a
// token) and writes to Cosmos as a side effect: on first call it seeds the demo
// Pro profile, the Westminster watch zone, and five fixed applications. Both
// APIs share the same Cosmos account and the seed is gated on the profile's
// absence, so the call is idempotent — whichever API runs first provisions the
// data and both then observe the identical account. The .NET response is the
// source of truth.
func TestContract_DemoAccount(t *testing.T) {
	dotnetURL := baseURL(t, "DOTNET_BASE_URL")
	goURL := baseURL(t, "GO_BASE_URL")
	client := &http.Client{Timeout: requestTimeout}

	// Prime both APIs once so the (one-time) seed write completes before the
	// diff: the first caller may seed while the second reads, and a freshly
	// seeded vs already-seeded response is identical, so a single warm-up call to
	// each removes any first-call seeding race from the comparison.
	get(t, client, dotnetURL+"/v1/demo-account")
	get(t, client, goURL+"/v1/demo-account")

	diffPath(t, client, dotnetURL, goURL, "/v1/demo-account")
}
