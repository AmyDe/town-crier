//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

// Iteration-4 contract scenarios: device tokens and notification state. All
// scenarios are sequenced to leave the shared integration-test user's data as
// they found it (the PUT/DELETE pair restores the device-token set; the
// notification-state writes only move the user's own watermark, which the
// .NET-side state was already moving on every mark-all-read in normal app use).

// TestContract_DeviceTokenLifecycle diffs the PUT -> DELETE round trip. The
// .NET call runs first (create), the Go call second (refresh of the same
// token) — both must answer an identical bodyless 204. The DELETE pair then
// diffs remove (on .NET) against idempotent re-remove (on Go), also 204s.
func TestContract_DeviceTokenLifecycle(t *testing.T) {
	dotnetURL := baseURL(t, "DOTNET_BASE_URL")
	goURL := baseURL(t, "GO_BASE_URL")
	token := integrationToken(t)
	client := &http.Client{Timeout: requestTimeout}

	const body = `{"token":"contract-test-token-it4","platform":"ios"}`

	t.Run("PUT registers", func(t *testing.T) {
		want := authedBodyRequest(t, client, dotnetURL, http.MethodPut, "/v1/me/device-token", token, body)
		got := authedBodyRequest(t, client, goURL, http.MethodPut, "/v1/me/device-token", token, body)
		diffAuthedShort(t, got, want)
	})

	t.Run("PUT rejects unknown platform", func(t *testing.T) {
		const bad = `{"token":"contract-test-token-it4","platform":"spectrum"}`
		want := authedBodyRequest(t, client, dotnetURL, http.MethodPut, "/v1/me/device-token", token, bad)
		got := authedBodyRequest(t, client, goURL, http.MethodPut, "/v1/me/device-token", token, bad)
		diffAuthedShort(t, got, want)
	})

	t.Run("DELETE removes idempotently", func(t *testing.T) {
		want := authedRequest(t, client, dotnetURL, http.MethodDelete, "/v1/me/device-token/contract-test-token-it4", token)
		got := authedRequest(t, client, goURL, http.MethodDelete, "/v1/me/device-token/contract-test-token-it4", token)
		diffAuthedShort(t, got, want)
	})
}

// TestContract_NotificationState diffs the read-watermark surface. The
// sequencing keeps every diffed call state-deterministic: an un-diffed .NET GET
// seeds the first-touch document if needed; mark-all-read responses are
// bodyless on both sides regardless of the watermark value; the advance uses a
// fixed past instant so it is a no-op against both implementations; and the
// trailing GETs read whatever the latest write left, identically for both.
func TestContract_NotificationState(t *testing.T) {
	dotnetURL := baseURL(t, "DOTNET_BASE_URL")
	goURL := baseURL(t, "GO_BASE_URL")
	token := integrationToken(t)
	client := &http.Client{Timeout: requestTimeout}

	// Seed, not a diff: guarantee the watermark document exists so neither
	// diffed GET takes the first-touch write path at a different instant.
	_ = authedRequest(t, client, dotnetURL, http.MethodGet, "/v1/me/notification-state", token)

	diffState := func(t *testing.T) {
		t.Helper()
		want := authedRequest(t, client, dotnetURL, http.MethodGet, "/v1/me/notification-state", token)
		got := authedRequest(t, client, goURL, http.MethodGet, "/v1/me/notification-state", token)
		diffAuthedShort(t, got, want)
	}

	t.Run("GET state", diffState)

	t.Run("POST mark-all-read", func(t *testing.T) {
		want := authedBodyRequest(t, client, dotnetURL, http.MethodPost, "/v1/me/notification-state/mark-all-read", token, "")
		got := authedBodyRequest(t, client, goURL, http.MethodPost, "/v1/me/notification-state/mark-all-read", token, "")
		diffAuthedShort(t, got, want)
	})

	t.Run("GET state after mark-all-read", diffState)

	t.Run("POST advance with stale asOf is a no-op", func(t *testing.T) {
		const body = `{"asOf":"2020-01-01T00:00:00+00:00"}`
		want := authedBodyRequest(t, client, dotnetURL, http.MethodPost, "/v1/me/notification-state/advance", token, body)
		got := authedBodyRequest(t, client, goURL, http.MethodPost, "/v1/me/notification-state/advance", token, body)
		diffAuthedShort(t, got, want)
	})

	t.Run("POST advance rejects malformed body", func(t *testing.T) {
		const body = `{"asOf":`
		want := authedBodyRequest(t, client, dotnetURL, http.MethodPost, "/v1/me/notification-state/advance", token, body)
		got := authedBodyRequest(t, client, goURL, http.MethodPost, "/v1/me/notification-state/advance", token, body)
		diffAuthedShort(t, got, want)
	})

	t.Run("GET state after advance", diffState)
}

// diffAuthedShort asserts status, content type, body, and rate-limit-header
// presence parity with truncated error output (long lines truncate CI logs).
func diffAuthedShort(t *testing.T, got, want authedResponse) {
	t.Helper()

	if got.status != want.status {
		t.Errorf("status: go=%d dotnet=%d", got.status, want.status)
	}
	if got.contentType != want.contentType {
		t.Errorf("content-type: go=%q dotnet=%q", got.contentType, want.contentType)
	}
	switch {
	case len(want.body) == 0 || len(got.body) == 0:
		if !bytes.Equal(got.body, want.body) {
			t.Errorf("body: go=%s dotnet=%s", truncate(got.body), truncate(want.body))
		}
	case !jsonEqual(t, got.body, want.body):
		t.Errorf("body: go=%s dotnet=%s", truncate(got.body), truncate(want.body))
	}
	if got.rateLimit != want.rateLimit {
		t.Errorf("X-RateLimit-Limit: go=%q dotnet=%q", got.rateLimit, want.rateLimit)
	}
}

func truncate(b []byte) string {
	const max = 512
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "...(truncated)"
}

// authedBodyRequest is authedRequest with a JSON request body.
func authedBodyRequest(t *testing.T, client *http.Client, base, method, path, token, body string) authedResponse {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, base+path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request %s %s: %v", method, path, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		t.Fatalf("read body %s %s: %v", method, path, err)
	}

	return authedResponse{
		status:        resp.StatusCode,
		contentType:   resp.Header.Get("Content-Type"),
		body:          raw,
		rateLimit:     resp.Header.Get("X-RateLimit-Limit"),
		rateRemaining: resp.Header.Get("X-RateLimit-Remaining"),
	}
}
