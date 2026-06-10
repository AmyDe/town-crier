//go:build e2e

// Package e2e holds black-box contract-diff tests. Every scenario executes
// against BOTH the deployed .NET API (DOTNET_BASE_URL) and the deployed Go
// API (GO_BASE_URL) and diffs the responses. The .NET response is always the
// expected value — parity is observed, never assumed from the source.
//
// Run: go test -tags e2e ./tests/e2e/...
// Tests skip when the URLs are unset so a plain `go test ./...` stays green
// locally.
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"reflect"
	"testing"
	"time"
)

// Generous timeout: both apps scale to zero in dev, so the first request may
// absorb an Azure Container Apps cold start.
const requestTimeout = 60 * time.Second

func TestContract_Health(t *testing.T) {
	dotnetURL := baseURL(t, "DOTNET_BASE_URL")
	goURL := baseURL(t, "GO_BASE_URL")
	client := &http.Client{Timeout: requestTimeout}

	for _, path := range []string{"/health", "/v1/health"} {
		t.Run(path, func(t *testing.T) {
			diffPath(t, client, dotnetURL, goURL, path)
		})
	}
}

// TestContract_EmbeddedResources diffs the iteration-1 embedded-resource
// endpoints (version-config, legal, authorities) including their error paths.
// The .NET response is always the source of truth.
func TestContract_EmbeddedResources(t *testing.T) {
	dotnetURL := baseURL(t, "DOTNET_BASE_URL")
	goURL := baseURL(t, "GO_BASE_URL")
	client := &http.Client{Timeout: requestTimeout}

	paths := []string{
		"/v1/version-config",

		// Legal: documents, case-insensitive lookup, and unknown -> 404.
		"/v1/legal/privacy",
		"/v1/legal/PRIVACY",
		"/v1/legal/terms",
		"/v1/legal/unknown",

		// Authorities: full list, search filter (substring, case-insensitive),
		// no-match empty array, trailing-slash list, blank/whitespace search.
		"/v1/authorities",
		"/v1/authorities/",
		"/v1/authorities?search=aberdeen",
		"/v1/authorities?search=ABERDEEN",
		"/v1/authorities?search=ZZZNOMATCH",
		"/v1/authorities?search=",

		// Authority by id: existing record and valid-int-but-missing -> 404.
		"/v1/authorities/384",
		"/v1/authorities/99999999",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			diffPath(t, client, dotnetURL, goURL, path)
		})
	}
}

// TestContract_AuthorityNonIntID is deferred to iteration 2. The .NET
// {id:int} route constraint rejects a non-integer id, which falls through to
// the auth fallback policy and returns 401 with WWW-Authenticate: Bearer.
// Iteration 1 ships no auth/error-backfill middleware, so this scenario is
// pinned here as a skip and enabled once iteration 2 lands.
func TestContract_AuthorityNonIntID(t *testing.T) {
	t.Skip("non-int authority id -> 401 depends on iteration-2 auth fallback middleware")
}

// diffPath fetches a path from both APIs and asserts status, content type, and
// JSON body match. Bodyless responses (e.g. iteration-1 404s) are compared as
// raw bytes since they are not valid JSON.
func diffPath(t *testing.T, client *http.Client, dotnetURL, goURL, path string) {
	t.Helper()

	want := get(t, client, dotnetURL+path)
	got := get(t, client, goURL+path)

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
		return
	}
	if !jsonEqual(t, got.body, want.body) {
		t.Errorf("body: go=%s dotnet=%s", got.body, want.body)
	}
}

type response struct {
	status      int
	contentType string
	body        []byte
}

func get(t *testing.T, client *http.Client, url string) response {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request %s: %v", url, err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("get %s: %v", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		t.Fatalf("read body %s: %v", url, err)
	}

	return response{
		status:      resp.StatusCode,
		contentType: resp.Header.Get("Content-Type"),
		body:        body,
	}
}

// jsonEqual compares two JSON payloads structurally, ignoring key order.
func jsonEqual(t *testing.T, a, b []byte) bool {
	t.Helper()

	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		t.Fatalf("unmarshal %q: %v", a, err)
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		t.Fatalf("unmarshal %q: %v", b, err)
	}
	return reflect.DeepEqual(av, bv)
}

func baseURL(t *testing.T, key string) string {
	t.Helper()

	v := os.Getenv(key)
	if v == "" {
		t.Skipf("%s not set — contract tests run in CI against deployed revisions", key)
	}
	return v
}
