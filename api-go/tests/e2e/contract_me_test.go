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
	return cachedToken
}

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

	// Warm-up, not a diff: the first authenticated request on a cold app pays
	// the lazy Cosmos connect + AAD token fetch, which can blow the bounded
	// 1.5s retry budget and make the rate-limit tier lookup fail open to the
	// free limit (observed on PR #424: go=60 vs dotnet=600 on the first diffed
	// call only). Both implementations share that fail-open design, so a cold
	// first call is an environmental artifact, not a contract difference.
	_ = authedRequest(t, client, dotnetURL, http.MethodGet, "/v1/me", token)
	_ = authedRequest(t, client, goURL, http.MethodGet, "/v1/me", token)

	scenarios := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/v1/me"}, // both now on the profile-exists path
		{http.MethodGet, "/api/me"},
		{http.MethodGet, "/v1/me"},
		{http.MethodGet, "/v1/me/data"},
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
