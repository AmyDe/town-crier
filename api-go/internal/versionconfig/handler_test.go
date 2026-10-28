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

	tests := []struct {
		name  string
		query string
		want  string
	}{
		{name: "no platform defaults to iOS", query: "", want: `{"minimumVersion":"1.2.11"}`},
		{name: "ios", query: "?platform=ios", want: `{"minimumVersion":"1.2.11"}`},
		{name: "android", query: "?platform=android", want: `{"minimumVersion":"1.0.0"}`},
		{name: "unknown platform defaults to iOS", query: "?platform=web", want: `{"minimumVersion":"1.2.11"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/v1/version-config"+tt.query, nil)
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
			if got, want := resp.Header.Get("Content-Type"), "application/json; charset=utf-8"; got != want {
				t.Errorf("content-type: got %q, want %q", got, want)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			if got := string(body); got != tt.want {
				t.Errorf("body: got %s, want %s", got, tt.want)
			}
		})
	}
}
