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
			want := get(t, client, dotnetURL+path)
			got := get(t, client, goURL+path)

			if got.status != want.status {
				t.Errorf("status: go=%d dotnet=%d", got.status, want.status)
			}
			if got.contentType != want.contentType {
				t.Errorf("content-type: go=%q dotnet=%q", got.contentType, want.contentType)
			}
			if !jsonEqual(t, got.body, want.body) {
				t.Errorf("body: go=%s dotnet=%s", got.body, want.body)
			}
		})
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
