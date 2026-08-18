package watchzones

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newCatalogServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	CatalogRoutes(mux, slog.New(slog.DiscardHandler))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestCatalogRoutes_FilterCatalog_StableOrder(t *testing.T) {
	t.Parallel()

	srv := newCatalogServer(t)

	// Call repeatedly: map iteration order is randomised per-process, so a
	// regression to iterating filterCatalog directly would show up as
	// flakiness across these calls even within a single test run.
	for i := range 5 {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/v1/watch-zones/filter-catalog", nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatalf("get: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var body struct {
			Filters []struct {
				Key         string `json:"key"`
				DisplayName string `json:"displayName"`
				Description string `json:"description"`
			} `json:"filters"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		resp.Body.Close()

		if len(body.Filters) != len(filterCatalogOrder) {
			t.Fatalf("filters length: got %d, want %d", len(body.Filters), len(filterCatalogOrder))
		}
		for idx, key := range filterCatalogOrder {
			def, ok := FilterDefinitionFor(key)
			if !ok {
				t.Fatalf("filterCatalogOrder[%d] = %q not in catalog", idx, key)
			}
			got := body.Filters[idx]
			if got.Key != string(key) {
				t.Errorf("iteration %d, entry %d: key: got %q, want %q", i, idx, got.Key, key)
			}
			if got.DisplayName != def.DisplayName {
				t.Errorf("iteration %d, entry %d: displayName: got %q, want %q", i, idx, got.DisplayName, def.DisplayName)
			}
			if got.Description != def.Description {
				t.Errorf("iteration %d, entry %d: description: got %q, want %q", i, idx, got.Description, def.Description)
			}
		}
	}
}

func TestCatalogRoutes_FilterCatalog_NoExpressionLeaked(t *testing.T) {
	t.Parallel()

	srv := newCatalogServer(t)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/v1/watch-zones/filter-catalog", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		t.Fatalf("decode top-level: %v", err)
	}

	filtersRaw, ok := raw["filters"]
	if !ok {
		t.Fatalf("response missing top-level %q field", "filters")
	}

	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(filtersRaw, &entries); err != nil {
		t.Fatalf("decode filters array: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("filters: got 0 entries, want > 0")
	}

	for i, entry := range entries {
		for field := range entry {
			lower := strings.ToLower(field)
			if strings.Contains(lower, "expression") || strings.Contains(lower, "tsquery") || strings.Contains(lower, "match") {
				t.Errorf("entry %d exposes matching-internals field %q", i, field)
			}
		}
		allowed := map[string]bool{"key": true, "displayName": true, "description": true}
		for field := range entry {
			if !allowed[field] {
				t.Errorf("entry %d has unexpected field %q", i, field)
			}
		}
	}
}

func TestCatalogRoutes_FilterCatalog_AnonymousNoAuthRequired(t *testing.T) {
	t.Parallel()

	// Call the handler directly with no auth context set on the request,
	// mirroring the pattern legal.Routes' tests use for their anonymous
	// route: this endpoint must never require an authenticated subject.
	mux := http.NewServeMux()
	CatalogRoutes(mux, slog.New(slog.DiscardHandler))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/watch-zones/filter-catalog", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestCatalogRoutes_FilterCatalog_CacheControlHeader(t *testing.T) {
	t.Parallel()

	srv := newCatalogServer(t)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/v1/watch-zones/filter-catalog", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	const want = "public, max-age=3600"
	if got := resp.Header.Get("Cache-Control"); got != want {
		t.Errorf("Cache-Control: got %q, want %q", got, want)
	}
}
