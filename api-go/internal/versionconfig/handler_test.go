package versionconfig

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRoutes_VersionConfig(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	Routes(mux, slog.New(slog.DiscardHandler))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/v1/version-config", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
	// Parity: .NET's Results.Ok serializes via the web JSON options (camelCase)
	// with the ASP.NET Core JSON content type including the charset parameter.
	if got, want := resp.Header.Get("Content-Type"), "application/json; charset=utf-8"; got != want {
		t.Errorf("content-type: got %q, want %q", got, want)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	// Exact wire bytes captured from the .NET dev API: camelCase, no whitespace.
	if got, want := string(body), `{"minimumVersion":"1.0.0"}`; got != want {
		t.Errorf("body: got %s, want %s", got, want)
	}
}
