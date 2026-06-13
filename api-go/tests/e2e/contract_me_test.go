//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"sync"
	"testing"
)

// Valid-token contract scenarios for the iteration-3 profile surface. The
// token is minted exactly as the .NET integration tests do
// (Auth0TokenProvider.cs): a password grant for the shared integration-test
// user. Scenarios skip when the INTEGRATION_TEST_* env is absent so plain
// local runs stay green.
//
// DELETE /v1/me is deliberately NOT covered: it removes the Cosmos profile AND
// the Auth0 user (M2M side effect), which would destroy the shared test user
// for every subsequent CI run. The .NET integration tests make the same
// choice. Its handler logic is covered by unit tests with fakes.

var (
	tokenOnce   sync.Once
	cachedToken string
	tokenErr    error
)

// integrationToken returns a cached access token for the integration-test
// user, skipping the test when the creds are not configured.
func integrationToken(t *testing.T) string {
	t.Helper()

	domain := os.Getenv("INTEGRATION_TEST_AUTH0_DOMAIN")
	clientID := os.Getenv("INTEGRATION_TEST_AUTH0_CLIENT_ID")
	audience := os.Getenv("INTEGRATION_TEST_AUTH0_AUDIENCE")
	username := os.Getenv("INTEGRATION_TEST_USERNAME")
	password := os.Getenv("INTEGRATION_TEST_PASSWORD")
	if domain == "" || clientID == "" || audience == "" || username == "" || password == "" {
		t.Skip("INTEGRATION_TEST_* env not set — valid-token contract scenarios run in CI")
	}

	tokenOnce.Do(func() {
		payload := map[string]string{
			"grant_type": "password",
			"client_id":  clientID,
			"username":   username,
			"password":   password,
			"audience":   audience,
			"scope":      "openid",
		}
		if secret := os.Getenv("INTEGRATION_TEST_AUTH0_CLIENT_SECRET"); secret != "" {
			payload["client_secret"] = secret
		}
		body, err := json.Marshal(payload)
		if err != nil {
			tokenErr = fmt.Errorf("marshal token payload: %w", err)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost,
			"https://"+domain+"/oauth/token", bytes.NewReader(body))
		if err != nil {
			tokenErr = fmt.Errorf("build token request: %w", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: requestTimeout}
		resp, err := client.Do(req)
		if err != nil {
			tokenErr = fmt.Errorf("token request: %w", err)
			return
		}
		defer resp.Body.Close()

		raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			tokenErr = fmt.Errorf("read token response: %w", err)
			return
		}
		if resp.StatusCode != http.StatusOK {
			tokenErr = fmt.Errorf("token request failed (%d): %s", resp.StatusCode, raw)
			return
		}
		var tokenResp struct {
			AccessToken string `json:"access_token"`
		}
		if err := json.Unmarshal(raw, &tokenResp); err != nil {
			tokenErr = fmt.Errorf("decode token response: %w", err)
			return
		}
		cachedToken = tokenResp.AccessToken
	})

	if tokenErr != nil {
		t.Fatalf("mint integration token: %v", tokenErr)
	}
	if cachedToken == "" {
		t.Fatal("mint integration token: empty access_token")
	}

	// Warm-up, not a diff, once per run and BEFORE whichever authed scenario
	// happens to execute first: the first authenticated request on a cold app
	// pays the lazy Cosmos connect + AAD token fetch, which can blow the
	// bounded 1.5s retry budget and make the rate-limit tier lookup fail open
	// to the free limit (PR #424 round 1 and PR #426 round 1: go=60 vs
	// dotnet=600 on the first diffed call only). Both implementations share
	// that fail-open design, so a cold first call is an environmental
	// artifact, not a contract difference.
	warmOnce.Do(func() {
		client := &http.Client{Timeout: requestTimeout}
		for _, base := range []string{os.Getenv("DOTNET_BASE_URL"), os.Getenv("GO_BASE_URL")} {
			if base != "" {
				_ = authedRequest(t, client, base, http.MethodGet, "/v1/me", cachedToken)
			}
		}
	})

	return cachedToken
}

var warmOnce sync.Once

// TestContract_AuthenticatedProfileSurface diffs the /api/me + /v1/me read and
// idempotent-write surface with a real token. The profile is first ensured to
// exist via the .NET API (un-diffed setup), so every diffed call hits the same
// deterministic profile-exists state on both implementations — they share one
// Cosmos database, making body equality exact.
func TestContract_AuthenticatedProfileSurface(t *testing.T) {
	dotnetURL := baseURL(t, "DOTNET_BASE_URL")
	goURL := baseURL(t, "GO_BASE_URL")
	token := integrationToken(t)
	client := &http.Client{Timeout: requestTimeout}

	// Setup, not a diff: create-or-already-exists on the .NET side so the
	// diffed POST below compares the exists path on both APIs.
	setup := authedRequest(t, client, dotnetURL, http.MethodPost, "/v1/me", token)
	if setup.status >= 500 {
		t.Fatalf("setup POST /v1/me on .NET failed: %d %s", setup.status, setup.body)
	}

	scenarios := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/v1/me"}, // both now on the profile-exists path
		{http.MethodGet, "/api/me"},
		{http.MethodGet, "/v1/me"},
	}
	for _, sc := range scenarios {
		t.Run(sc.method+" "+sc.path, func(t *testing.T) {
			want := authedRequest(t, client, dotnetURL, sc.method, sc.path, token)
			got := authedRequest(t, client, goURL, sc.method, sc.path, token)

			if got.status != want.status {
				t.Errorf("status: go=%d dotnet=%d", got.status, want.status)
			}
			if got.contentType != want.contentType {
				t.Errorf("content-type: go=%q dotnet=%q", got.contentType, want.contentType)
			}
			if len(want.body) == 0 || len(got.body) == 0 {
				if !bytes.Equal(got.body, want.body) {
					t.Errorf("body: go=%q dotnet=%q", got.body, want.body)
				}
			} else if !jsonEqual(t, got.body, want.body) {
				t.Errorf("body: go=%s dotnet=%s", got.body, want.body)
			}

			// Rate limiting must be active on both implementations for
			// authenticated routes. The remaining budget legitimately differs
			// (each app keeps its own in-memory window), so only the limit —
			// derived from the same Cosmos profile tier — is diffed for
			// equality.
			if got.rateLimit != want.rateLimit {
				t.Errorf("X-RateLimit-Limit: go=%q dotnet=%q", got.rateLimit, want.rateLimit)
			}
			if got.rateLimit != "" && got.rateRemaining == "" {
				t.Error("go: X-RateLimit-Remaining missing while X-RateLimit-Limit present")
			}
		})
	}
}

// TestContract_GDPRExportProfileFields diffs the profile-derived portion of
// GET /v1/me/data. Full-body equality is deferred until the collection stores
// land (devices it4, watch zones it5, saved applications it6, offer codes
// it8 — re-enable in tc-7g3i.9): until then the Go export serialises those
// collections as empty arrays while the live .NET API returns the integration
// user's real records, so a whole-body diff can only fail. The profile-derived
// fields read the same Cosmos document on both sides and must match exactly;
// the collection keys must at least be present with array values.
func TestContract_GDPRExportProfileFields(t *testing.T) {
	dotnetURL := baseURL(t, "DOTNET_BASE_URL")
	goURL := baseURL(t, "GO_BASE_URL")
	token := integrationToken(t)
	client := &http.Client{Timeout: requestTimeout}

	want := authedRequest(t, client, dotnetURL, http.MethodGet, "/v1/me/data", token)
	got := authedRequest(t, client, goURL, http.MethodGet, "/v1/me/data", token)

	if got.status != want.status {
		t.Fatalf("status: go=%d dotnet=%d", got.status, want.status)
	}
	if got.contentType != want.contentType {
		t.Errorf("content-type: go=%q dotnet=%q", got.contentType, want.contentType)
	}

	var wantDoc, gotDoc map[string]json.RawMessage
	if err := json.Unmarshal(want.body, &wantDoc); err != nil {
		t.Fatalf("decode dotnet export: %v", err)
	}
	if err := json.Unmarshal(got.body, &gotDoc); err != nil {
		t.Fatalf("decode go export: %v", err)
	}

	for _, key := range []string{"userId", "email", "subscription"} {
		if !jsonEqual(t, gotDoc[key], wantDoc[key]) {
			t.Errorf("%s: go=%s dotnet=%s", key, gotDoc[key], wantDoc[key])
		}
	}
	// notificationPreferences carries the zonePreferences array, whose order is
	// not a contract guarantee — neither API issues an ORDER BY, so the shared
	// Cosmos returns the zones in an order that varies between requests. Sort by
	// zoneId before diffing so the comparison is about content, not order
	// (tc-zgnt).
	if !jsonEqual(t, sortZonePreferences(t, gotDoc["notificationPreferences"]), sortZonePreferences(t, wantDoc["notificationPreferences"])) {
		t.Errorf("notificationPreferences: go=%s dotnet=%s", gotDoc["notificationPreferences"], wantDoc["notificationPreferences"])
	}
	for _, key := range []string{"watchZones", "notifications", "savedApplications", "deviceRegistrations", "offerCodeRedemptions"} {
		if raw, ok := gotDoc[key]; !ok || len(raw) == 0 || raw[0] != '[' {
			t.Errorf("go export %q: missing or not an array (%s)", key, raw)
		}
		if raw, ok := wantDoc[key]; !ok || len(raw) == 0 || raw[0] != '[' {
			t.Errorf("dotnet export %q: missing or not an array (%s)", key, raw)
		}
	}
}

// sortZonePreferences returns the notificationPreferences object with its
// zonePreferences array sorted by zoneId, so an order-undefined array can be
// diffed for content. A payload without a zonePreferences array is returned
// unchanged.
func sortZonePreferences(t *testing.T, raw json.RawMessage) []byte {
	t.Helper()
	if len(raw) == 0 {
		return raw
	}
	var prefs map[string]json.RawMessage
	if err := json.Unmarshal(raw, &prefs); err != nil {
		t.Fatalf("decode notificationPreferences: %v", err)
	}
	zonesRaw, ok := prefs["zonePreferences"]
	if !ok {
		return raw
	}
	var zones []map[string]json.RawMessage
	if err := json.Unmarshal(zonesRaw, &zones); err != nil {
		t.Fatalf("decode zonePreferences: %v", err)
	}
	sort.Slice(zones, func(i, j int) bool {
		return string(zones[i]["zoneId"]) < string(zones[j]["zoneId"])
	})
	sorted, err := json.Marshal(zones)
	if err != nil {
		t.Fatalf("marshal zonePreferences: %v", err)
	}
	prefs["zonePreferences"] = sorted
	out, err := json.Marshal(prefs)
	if err != nil {
		t.Fatalf("marshal notificationPreferences: %v", err)
	}
	return out
}

// authedResponse is one authenticated exchange, including the rate-limit
// headers the anonymous helper has no use for.
type authedResponse struct {
	status        int
	contentType   string
	body          []byte
	rateLimit     string
	rateRemaining string
}

func authedRequest(t *testing.T, client *http.Client, base, method, path, token string) authedResponse {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, base+path, nil)
	if err != nil {
		t.Fatalf("new request %s %s: %v", method, path, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		t.Fatalf("read body %s %s: %v", method, path, err)
	}

	return authedResponse{
		status:        resp.StatusCode,
		contentType:   resp.Header.Get("Content-Type"),
		body:          body,
		rateLimit:     resp.Header.Get("X-RateLimit-Limit"),
		rateRemaining: resp.Header.Get("X-RateLimit-Remaining"),
	}
}
