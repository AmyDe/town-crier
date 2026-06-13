//go:build e2e

package e2e

import (
	"net/http"
	"testing"
)

// Valid-token contract scenarios for the iteration-7 geocode and designation
// surface. Both endpoints are authed and call live UK upstreams (postcodes.io
// and planning.data.gov.uk). The .NET and Go APIs hit the same upstreams, so the
// responses are deterministic and body-equal; the .NET response is the source of
// truth. Scenarios reuse the authed watch-zone diff helper and skip when the
// integration-token env is absent, so a plain local run stays green.

func TestContract_Geocode(t *testing.T) {
	dotnetURL := baseURL(t, "DOTNET_BASE_URL")
	goURL := baseURL(t, "GO_BASE_URL")
	token := integrationToken(t)
	client := &http.Client{Timeout: requestTimeout}

	// A real postcode resolves to deterministic coordinates from postcodes.io.
	t.Run("valid postcode", func(t *testing.T) {
		diffWatchZone(t, client, dotnetURL, goURL, http.MethodGet, "/v1/geocode/SW1A%201AA", token, "")
	})

	// A malformed postcode is rejected at the boundary with the ApiErrorResponse
	// envelope (400), before any upstream call.
	t.Run("invalid postcode", func(t *testing.T) {
		diffWatchZone(t, client, dotnetURL, goURL, http.MethodGet, "/v1/geocode/NOTAPOSTCODE", token, "")
	})

	// A valid-format postcode that does not exist: postcodes.io 404s, so the
	// endpoint returns 404 with the ApiErrorResponse envelope.
	t.Run("valid format, nonexistent", func(t *testing.T) {
		diffWatchZone(t, client, dotnetURL, goURL, http.MethodGet, "/v1/geocode/ZZ1%201ZZ", token, "")
	})
}

func TestContract_Designations(t *testing.T) {
	dotnetURL := baseURL(t, "DOTNET_BASE_URL")
	goURL := baseURL(t, "GO_BASE_URL")
	token := integrationToken(t)
	client := &http.Client{Timeout: requestTimeout}

	// A North Sea point (lat 55, lng 2) intersects no designated entity, so
	// planning.data.gov.uk 404s and both APIs return the empty designation
	// context (all false / null).
	t.Run("sea point yields none", func(t *testing.T) {
		diffWatchZone(t, client, dotnetURL, goURL, http.MethodGet, "/v1/designations?latitude=55&longitude=2", token, "")
	})
}
