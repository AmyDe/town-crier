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

// fakeNearPointStore is a hand-written nearPointStore fake that records the
// last call's arguments and returns caller-configured results.
type fakeNearPointStore struct {
	apps       []PlanningApplication
	nextCursor string
	err        error

	calledLat    float64
	calledLng    float64
	calledRadius float64
	calledLimit  int
	calledCursor string
}

func (f *fakeNearPointStore) FindNearbyPage(_ context.Context, latitude, longitude, radiusMetres float64, limit int, cursor string) ([]PlanningApplication, string, error) {
	f.calledLat = latitude
	f.calledLng = longitude
	f.calledRadius = radiusMetres
	f.calledLimit = limit
	f.calledCursor = cursor
	if f.err != nil {
		return nil, "", f.err
	}
	return f.apps, f.nextCursor, nil
}

// newNearPointTestHandler builds a near-point mux wired to the given fake
// store, discarding log output.
func newNearPointTestHandler(store *fakeNearPointStore) http.Handler {
	mux := http.NewServeMux()
	NearPointRoutes(mux, store, slog.New(slog.DiscardHandler))
	return mux
}

// TestNearPointHandler_RequiresLatLng proves a missing or unparseable lat/lng
// is a bodyless 400 and never reaches the store.
func TestNearPointHandler_RequiresLatLng(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
	}{
		{"missing both", "/v1/applications/near-point"},
		{"missing lng", "/v1/applications/near-point?lat=51.5"},
		{"missing lat", "/v1/applications/near-point?lng=-0.1"},
		{"non-numeric lat", "/v1/applications/near-point?lat=abc&lng=-0.1"},
		{"non-numeric lng", "/v1/applications/near-point?lat=51.5&lng=xyz"},
		{"lat out of range high", "/v1/applications/near-point?lat=91&lng=-0.1"},
		{"lat out of range low", "/v1/applications/near-point?lat=-91&lng=-0.1"},
		{"lng out of range high", "/v1/applications/near-point?lat=51.5&lng=181"},
		{"lng out of range low", "/v1/applications/near-point?lat=51.5&lng=-181"},
		{"lat is NaN literal", "/v1/applications/near-point?lat=NaN&lng=-0.1"},
		{"lat is Inf literal", "/v1/applications/near-point?lat=Inf&lng=-0.1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store := &fakeNearPointStore{}
			h := newNearPointTestHandler(store)

			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			if rec.Body.Len() != 0 {
				t.Errorf("body = %q, want empty (bodyless 400)", rec.Body.String())
			}
		})
	}
}

// TestNearPointHandler_ValidLatLngUsesDefaults proves a bare valid lat/lng
// call reaches the store with the documented defaults: radius 2000m, limit
// 100, no cursor.
func TestNearPointHandler_ValidLatLngUsesDefaults(t *testing.T) {
	t.Parallel()

	store := &fakeNearPointStore{}
	h := newNearPointTestHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/v1/applications/near-point?lat=51.5&lng=-0.1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	if store.calledLat != 51.5 || store.calledLng != -0.1 {
		t.Errorf("store called with (%v, %v), want (51.5, -0.1)", store.calledLat, store.calledLng)
	}
	if store.calledRadius != nearPointDefaultRadiusMetres {
		t.Errorf("radius = %v, want default %v", store.calledRadius, nearPointDefaultRadiusMetres)
	}
	if store.calledLimit != nearPointDefaultLimit {
		t.Errorf("limit = %v, want default %v", store.calledLimit, nearPointDefaultLimit)
	}
	if store.calledCursor != "" {
		t.Errorf("cursor = %q, want empty first page", store.calledCursor)
	}
}

// TestNearPointHandler_ResponseShape proves the body is a bare JSON array of
// NearbyResult (not wrapped in an envelope), matching the authed nearby page.
func TestNearPointHandler_ResponseShape(t *testing.T) {
	t.Parallel()

	app := PlanningApplication{Name: "24/0001/FUL", UID: "uid-1", AreaName: "Testshire", AreaID: 100, Address: "1 Test Street"}
	store := &fakeNearPointStore{apps: []PlanningApplication{app}}
	h := newNearPointTestHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/v1/applications/near-point?lat=51.5&lng=-0.1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}

	var results []NearbyResult
	if err := json.Unmarshal(rec.Body.Bytes(), &results); err != nil {
		t.Fatalf("body is not a bare JSON array: %v; body = %s", err, rec.Body.String())
	}
	if len(results) != 1 || results[0].Name != app.Name || results[0].UID != app.UID {
		t.Errorf("results = %+v, want one NearbyResult matching %+v", results, app)
	}
}

// TestNearPointHandler_StoreError proves a store failure is a bodyless 500.
func TestNearPointHandler_StoreError(t *testing.T) {
	t.Parallel()

	store := &fakeNearPointStore{err: errors.New("boom")}
	h := newNearPointTestHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/v1/applications/near-point?lat=51.5&lng=-0.1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want empty (bodyless 500)", rec.Body.String())
	}
}
