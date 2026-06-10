package health

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRoutes_HealthEndpoints(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	Routes(mux, slog.New(slog.DiscardHandler))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	tests := []struct {
		name string
		path string
	}{
		{"bare path", "/health"},
		{"versioned path", "/v1/health"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+tc.path, nil)
			if err != nil {
				t.Fatalf("new request %s: %v", tc.path, err)
			}
			resp, err := srv.Client().Do(req)
			if err != nil {
				t.Fatalf("get %s: %v", tc.path, err)
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
			if got, want := string(body), `{"status":"Healthy"}`; got != want {
				t.Errorf("body: got %s, want %s", got, want)
			}
		})
	}
}
