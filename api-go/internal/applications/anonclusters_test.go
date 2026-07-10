package applications

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeClustersStore is a hand-written clustersStore fake that records the last
// call's query and returns caller-configured results.
type fakeClustersStore struct {
	clusters []Cluster
	err      error

	calledQuery ClusterQuery
	called      bool
}

func (f *fakeClustersStore) FindClustersInZone(_ context.Context, q ClusterQuery) ([]Cluster, error) {
	f.called = true
	f.calledQuery = q
	if f.err != nil {
		return nil, f.err
	}
	return f.clusters, nil
}

// newClustersTestHandler builds an anonymous clusters mux wired to the given
// fake store and resolver, discarding log output.
func newClustersTestHandler(store *fakeClustersStore, resolver authoritySlugResolver) http.Handler {
	mux := http.NewServeMux()
	ClustersRoutes(mux, store, resolver, slog.New(slog.DiscardHandler))
	return mux
}

// validClustersQuery is a well-formed query string covering every required
// param, reused as a base by tests that only want to vary one thing.
const validClustersQuery = "?lat=51.5074&lng=-0.1278&bbox=-0.2,51.4,-0.05,51.6&zoom=14"

// TestClustersHandler_RequiresLatLng proves a missing or unparseable lat/lng is
// a bodyless 400 and never reaches the store.
func TestClustersHandler_RequiresLatLng(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
	}{
		{"missing both", "/v1/applications/clusters?bbox=-0.2,51.4,-0.05,51.6&zoom=14"},
		{"missing lng", "/v1/applications/clusters?lat=51.5&bbox=-0.2,51.4,-0.05,51.6&zoom=14"},
		{"missing lat", "/v1/applications/clusters?lng=-0.1&bbox=-0.2,51.4,-0.05,51.6&zoom=14"},
		{"non-numeric lat", "/v1/applications/clusters?lat=abc&lng=-0.1&bbox=-0.2,51.4,-0.05,51.6&zoom=14"},
		{"lat out of range", "/v1/applications/clusters?lat=91&lng=-0.1&bbox=-0.2,51.4,-0.05,51.6&zoom=14"},
		{"lat is NaN literal", "/v1/applications/clusters?lat=NaN&lng=-0.1&bbox=-0.2,51.4,-0.05,51.6&zoom=14"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			store := &fakeClustersStore{}
			h := newClustersTestHandler(store, testResolver())

			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, tc.url, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			if rec.Body.Len() != 0 {
				t.Errorf("body = %q, want empty (bodyless 400)", rec.Body.String())
			}
			if store.called {
				t.Error("store must not be called on invalid lat/lng")
			}
		})
	}
}

// TestClustersHandler_RequiresBBox proves a missing or malformed bbox is a
// bodyless 400 and never reaches the store.
func TestClustersHandler_RequiresBBox(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
	}{
		{"missing bbox", "/v1/applications/clusters?lat=51.5&lng=-0.1&zoom=14"},
		{"wrong field count", "/v1/applications/clusters?lat=51.5&lng=-0.1&bbox=-0.2,51.4,-0.05&zoom=14"},
		{"degenerate rectangle", "/v1/applications/clusters?lat=51.5&lng=-0.1&bbox=-0.05,51.4,-0.2,51.6&zoom=14"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			store := &fakeClustersStore{}
			h := newClustersTestHandler(store, testResolver())

			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, tc.url, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			if rec.Body.Len() != 0 {
				t.Errorf("body = %q, want empty (bodyless 400)", rec.Body.String())
			}
			if store.called {
				t.Error("store must not be called on invalid bbox")
			}
		})
	}
}

// TestClustersHandler_RequiresZoom proves a missing, non-integer, or
// out-of-range zoom is a bodyless 400 and never reaches the store.
func TestClustersHandler_RequiresZoom(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
	}{
		{"missing zoom", "/v1/applications/clusters?lat=51.5&lng=-0.1&bbox=-0.2,51.4,-0.05,51.6"},
		{"non-integer zoom", "/v1/applications/clusters?lat=51.5&lng=-0.1&bbox=-0.2,51.4,-0.05,51.6&zoom=abc"},
		{"negative zoom", "/v1/applications/clusters?lat=51.5&lng=-0.1&bbox=-0.2,51.4,-0.05,51.6&zoom=-1"},
		{"zoom above maxZoom", "/v1/applications/clusters?lat=51.5&lng=-0.1&bbox=-0.2,51.4,-0.05,51.6&zoom=21"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			store := &fakeClustersStore{}
			h := newClustersTestHandler(store, testResolver())

			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, tc.url, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			if rec.Body.Len() != 0 {
				t.Errorf("body = %q, want empty (bodyless 400)", rec.Body.String())
			}
			if store.called {
				t.Error("store must not be called on invalid zoom")
			}
		})
	}
}

// TestClustersHandler_UnknownStatusIs400 proves an unrecognised ?status= is a
// bodyless 400, while an absent status or "All" both mean no filter.
func TestClustersHandler_UnknownStatusIs400(t *testing.T) {
	t.Parallel()

	store := &fakeClustersStore{}
	h := newClustersTestHandler(store, testResolver())
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/applications/clusters"+validClustersQuery+"&status=NotARealStatus", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want empty (bodyless 400)", rec.Body.String())
	}
	if store.called {
		t.Error("store must not be called on invalid status")
	}
}

// TestClustersHandler_StatusAllIsNoFilter proves ?status=All and an absent
// ?status= both pass an empty Status filter through to the store.
func TestClustersHandler_StatusAllIsNoFilter(t *testing.T) {
	t.Parallel()

	tests := []string{"", "&status=All"}
	for _, suffix := range tests {
		store := &fakeClustersStore{}
		h := newClustersTestHandler(store, testResolver())
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/applications/clusters"+validClustersQuery+suffix, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("suffix=%q status = %d, want 200; body = %s", suffix, rec.Code, rec.Body.String())
		}
		if store.calledQuery.Status != "" {
			t.Errorf("suffix=%q status filter: got %q, want empty", suffix, store.calledQuery.Status)
		}
	}
}

// TestClustersHandler_RadiusClamping proves the optional ?radius= is clamped
// into [100, 5000] with a default of 2000, identical to near-point.
func TestClustersHandler_RadiusClamping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		suffix string
		want   float64
	}{
		{"absent defaults to 2000", "", 2000},
		{"below minimum clamps to 100", "&radius=10", 100},
		{"above maximum clamps to 5000", "&radius=99999", 5000},
		{"in range passes through", "&radius=3000", 3000},
		{"non-numeric defaults to 2000", "&radius=abc", 2000},
		{"negative clamps to 100", "&radius=-50", 100},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			store := &fakeClustersStore{}
			h := newClustersTestHandler(store, testResolver())
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/applications/clusters"+validClustersQuery+tc.suffix, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
			}
			if store.calledQuery.RadiusMetres != tc.want {
				t.Errorf("radius: got %v, want %v", store.calledQuery.RadiusMetres, tc.want)
			}
		})
	}
}

// TestClustersHandler_PassesParamsToStore proves a well-formed request
// translates every param into the ClusterQuery the store receives, including
// the finest-grid coalesce threshold (independent of the request's own zoom).
func TestClustersHandler_PassesParamsToStore(t *testing.T) {
	t.Parallel()

	store := &fakeClustersStore{}
	h := newClustersTestHandler(store, testResolver())
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet,
		"/v1/applications/clusters?lat=51.5074&lng=-0.1278&radius=2500&bbox=-0.2,51.4,-0.05,51.6&zoom=10&status=Permitted", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	q := store.calledQuery
	if q.Latitude != 51.5074 || q.Longitude != -0.1278 {
		t.Errorf("centre: got (%v, %v), want (51.5074, -0.1278)", q.Latitude, q.Longitude)
	}
	if q.RadiusMetres != 2500 {
		t.Errorf("radius: got %v, want 2500", q.RadiusMetres)
	}
	if q.West != -0.2 || q.South != 51.4 || q.East != -0.05 || q.North != 51.6 {
		t.Errorf("viewport: got %+v, want {-0.2, 51.4, -0.05, 51.6}", q)
	}
	wantGrid, ok := GridDegreesForZoom(10)
	if !ok || q.GridSizeDegrees != wantGrid {
		t.Errorf("grid size: got %v, want %v (zoom 10)", q.GridSizeDegrees, wantGrid)
	}
	if q.Status != "Permitted" {
		t.Errorf("status: got %q, want %q", q.Status, "Permitted")
	}
	if q.CoalesceThresholdDegrees != FinestGridDegrees() {
		t.Errorf("coalesce threshold: got %v, want %v (finest grid, independent of request zoom)", q.CoalesceThresholdDegrees, FinestGridDegrees())
	}
}

// TestClustersHandler_SlugEnrichment proves the handler populates AuthoritySlug
// on a single-member cell's Member and on every entry of a stacked cell's
// Members, via the resolver.
func TestClustersHandler_SlugEnrichment(t *testing.T) {
	t.Parallel()

	store := &fakeClustersStore{clusters: []Cluster{
		{ // single-member cell
			Latitude: 51.51, Longitude: -0.12, Count: 1,
			StatusCounts: map[string]int{"Permitted": 1},
			Member:       &PlanningApplicationID{Authority: "471", Name: "24/001"},
		},
		{ // unsplittable multi-member (stacked) cell
			Latitude: 51.52, Longitude: -0.13, Count: 2,
			StatusCounts: map[string]int{"Permitted": 2},
			Members: []PlanningApplicationID{
				{Authority: "471", Name: "24/002"},
				{Authority: "471", Name: "24/003"},
			},
		},
	}}
	h := newClustersTestHandler(store, testResolver())
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/applications/clusters"+validClustersQuery, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var got []Cluster
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v; raw=%s", err, rec.Body.String())
	}
	if len(got) != 2 {
		t.Fatalf("clusters: got %d, want 2", len(got))
	}
	if got[0].Member == nil || got[0].Member.AuthoritySlug != "city-of-london" {
		t.Errorf("single-member slug: got %+v, want AuthoritySlug=city-of-london", got[0].Member)
	}
	if len(got[1].Members) != 2 {
		t.Fatalf("stacked members: got %d, want 2", len(got[1].Members))
	}
	for i, m := range got[1].Members {
		if m.AuthoritySlug != "city-of-london" {
			t.Errorf("stacked member[%d] slug: got %q, want city-of-london", i, m.AuthoritySlug)
		}
	}
}

// TestClustersHandler_SlugEnrichmentMiss proves a resolver miss (area id absent
// from the static authorities table, or a malformed authority string) leaves
// AuthoritySlug empty rather than erroring the request.
func TestClustersHandler_SlugEnrichmentMiss(t *testing.T) {
	t.Parallel()

	store := &fakeClustersStore{clusters: []Cluster{
		{
			Latitude: 51.51, Longitude: -0.12, Count: 1,
			StatusCounts: map[string]int{"Permitted": 1},
			Member:       &PlanningApplicationID{Authority: "999999", Name: "24/999"}, // unknown area id
		},
		{
			Latitude: 51.53, Longitude: -0.14, Count: 1,
			StatusCounts: map[string]int{"Permitted": 1},
			Member:       &PlanningApplicationID{Authority: "not-a-number", Name: "24/998"}, // malformed
		},
	}}
	h := newClustersTestHandler(store, testResolver())
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/applications/clusters"+validClustersQuery, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var got []Cluster
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v; raw=%s", err, rec.Body.String())
	}
	if len(got) != 2 {
		t.Fatalf("clusters: got %d, want 2", len(got))
	}
	for _, c := range got {
		if c.Member == nil {
			t.Fatalf("expected a member on every cell, got nil: %+v", c)
		}
		if c.Member.AuthoritySlug != "" {
			t.Errorf("expected empty slug on a resolver miss, got %q", c.Member.AuthoritySlug)
		}
	}
}

// TestClustersHandler_EmptyResultEncodesEmptyArray proves a nil store result
// encodes as a bare JSON array, never null.
func TestClustersHandler_EmptyResultEncodesEmptyArray(t *testing.T) {
	t.Parallel()

	store := &fakeClustersStore{clusters: nil}
	h := newClustersTestHandler(store, testResolver())
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/applications/clusters"+validClustersQuery, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != "[]" {
		t.Errorf("body = %q, want \"[]\"", got)
	}
}

// TestClustersHandler_StoreErrorIs500 proves a store failure is a bodyless 500.
func TestClustersHandler_StoreErrorIs500(t *testing.T) {
	t.Parallel()

	store := &fakeClustersStore{err: errors.New("boom")}
	h := newClustersTestHandler(store, testResolver())
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/applications/clusters"+validClustersQuery, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want empty (bodyless 500)", rec.Body.String())
	}
}
